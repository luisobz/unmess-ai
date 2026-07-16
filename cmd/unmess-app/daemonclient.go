//go:build gui

// La app nativa habla con el daemon exclusivamente por su API HTTP local
// (127.0.0.1:<ui.port>). No comparte memoria con el daemon: lo trata como un
// servicio, igual que la UI web. Así el mismo binario funciona tanto si el
// daemon lo arranca un servicio del SO como si lo lanza la propia app.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/luisobz/unmess-ai/internal/daemon"
)

// daemonClient es un cliente del API local del daemon.
type daemonClient struct {
	baseURL string
	http    *http.Client

	// mu protege token, que se refresca si el daemon se reinicia y regenera el
	// token mientras la app sigue viva.
	mu    sync.Mutex
	token string
}

// fetchToken pide el token de sesión a un daemon ya en marcha. Sirve además como
// sonda de vida: si el daemon no responde, devuelve error.
func fetchToken(baseURL string) (string, error) {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(baseURL + "/api/token")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token: estado %d", resp.StatusCode)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.Token == "" {
		return "", fmt.Errorf("token vacío")
	}
	return body.Token, nil
}

// ensureDaemon garantiza que hay un daemon escuchando en el puerto dado y
// devuelve un cliente listo. Si no responde, arranca unmessd y espera a que
// levante. spawned indica si esta app arrancó el daemon (para poder pararlo al
// salir).
func ensureDaemon(port int) (client *daemonClient, spawned *exec.Cmd, err error) {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	if tok, terr := fetchToken(baseURL); terr == nil {
		return newClient(baseURL, tok), nil, nil
	}

	// No responde: arrancamos el daemon nosotros.
	cmd, serr := startDaemon()
	if serr != nil {
		return nil, nil, serr
	}

	// Esperamos a que el daemon escriba el token y abra el puerto.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if tok, terr := fetchToken(baseURL); terr == nil {
			return newClient(baseURL, tok), cmd, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = stopDaemon(cmd)
	return nil, nil, fmt.Errorf("el daemon no respondió en %s tras arrancarlo", baseURL)
}

func newClient(baseURL, token string) *daemonClient {
	return &daemonClient{
		baseURL: baseURL,
		token:   token,
		// Sin timeout global: el stream de eventos es de larga duración. Cada
		// petición puntual usa su propio contexto con timeout.
		http: &http.Client{},
	}
}

// currentToken devuelve el token actual de forma segura frente a concurrencia.
func (c *daemonClient) currentToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

// refreshToken vuelve a pedir el token al daemon y lo actualiza. Se usa cuando
// una petición devuelve 401: el daemon pudo reiniciarse (actualización, crash,
// `snap refresh`) y regenerar el token, dejando el que teníamos inválido.
func (c *daemonClient) refreshToken() error {
	tok, err := fetchToken(c.baseURL)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.token = tok
	c.mu.Unlock()
	return nil
}

// doAuthed hace una petición autenticada al daemon. Ante un 401 refresca el
// token una vez y reintenta, para sobrevivir a un reinicio del daemon sin tener
// que reiniciar la app.
func (c *daemonClient) doAuthed(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	send := func() (*http.Response, error) {
		var r io.Reader
		if body != nil {
			r = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.currentToken())
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return c.http.Do(req)
	}
	resp, err := send()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		if rerr := c.refreshToken(); rerr != nil {
			return nil, rerr
		}
		return send()
	}
	return resp, nil
}

// startDaemon localiza y lanza el binario unmessd.
func startDaemon() (*exec.Cmd, error) {
	path, err := findUnmessd()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	hideSpawnedConsole(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("arrancando %s: %w", path, err)
	}
	return cmd, nil
}

// stopDaemon termina un daemon que arrancó esta app.
func stopDaemon(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	// SIGTERM permite el apagado limpio del daemon (flush de pendientes).
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
	}
	done := make(chan struct{})
	go func() { _, _ = cmd.Process.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
	}
	return nil
}

// openArgFrom extrae el valor de -open/--open de una lista de argumentos (los
// de una segunda invocación de la app). Acepta "-open x", "--open x",
// "-open=x" y "--open=x".
func openArgFrom(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		for _, pfx := range []string{"--open=", "-open="} {
			if v, ok := strings.CutPrefix(a, pfx); ok {
				return v
			}
		}
		if (a == "-open" || a == "--open") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// findUnmessd busca el binario unmessd junto a la app y, si no, en el PATH.
func findUnmessd() (string, error) {
	name := "unmessd"
	if runtime.GOOS == "windows" {
		name = "unmessd.exe"
	}
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), name)
		if fi, serr := os.Stat(cand); serr == nil && !fi.IsDir() {
			return cand, nil
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("no se encontró %q junto a la app ni en el PATH", name)
}

// setPaused pausa o reanuda la vigilancia vía POST /api/pause.
func (c *daemonClient) setPaused(paused bool) error {
	body, _ := json.Marshal(map[string]bool{"paused": paused})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.doAuthed(ctx, http.MethodPost, "/api/pause", body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pause: estado %d", resp.StatusCode)
	}
	return nil
}

// status consulta GET /api/status (usado para conocer el estado inicial de
// pausa). Comprueba el código HTTP: un 401 (u otro no-200) debe ser un error,
// no decodificarse como si fuera un estado válido (el cuerpo de error tiene
// paused ausente → false, y silenciaría un fallo de autenticación).
func (c *daemonClient) status() (paused bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := c.doAuthed(ctx, http.MethodGet, "/api/status", nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("status: estado %d", resp.StatusCode)
	}
	var body struct {
		Paused bool `json:"paused"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, err
	}
	return body.Paused, nil
}

// streamEvents mantiene abierta la conexión SSE a /api/events y entrega cada
// evento a onEvent. Reconecta con backoff hasta que ctx se cancela.
func (c *daemonClient) streamEvents(ctx context.Context, onEvent func(daemon.Event)) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := c.readEventStream(ctx, onEvent)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			// La conexión estuvo viva y terminó limpia: reconecta rápido en vez
			// de arrastrar el backoff acumulado por caídas anteriores.
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 15*time.Second {
			backoff *= 2
		}
	}
}

// readEventStream lee un stream SSE hasta que se corta o ctx se cancela.
func (c *daemonClient) readEventStream(ctx context.Context, onEvent func(daemon.Event)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/events", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.currentToken())
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		// El daemon se reinició y regeneró el token: lo refrescamos para que la
		// siguiente reconexión (tras el backoff) use el token nuevo. Si no,
		// streamEvents reintentaría para siempre con el token viejo y la bandeja
		// nunca recuperaría el estado.
		_ = c.refreshToken()
		return fmt.Errorf("events: 401 (token refrescado)")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("events: estado %d", resp.StatusCode)
	}

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		// Solo nos interesan las líneas "data:"; el tipo va dentro del JSON.
		data, ok := strings.CutPrefix(line, "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "" {
			continue
		}
		var ev daemon.Event
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			continue
		}
		onEvent(ev)
	}
	return sc.Err()
}
