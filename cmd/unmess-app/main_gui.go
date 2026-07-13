//go:build gui

// Command unmess-app es la app nativa de unmessai: icono en la bandeja/menu bar
// del SO, ventana propia con WebView que carga la UI local, y notificaciones
// nativas de los eventos del daemon (versionado, restauración, error).
//
// Se compila con la etiqueta de build `gui` (requiere CGO + WebView del SO:
// WebKitGTK en Linux, WebView2 en Windows, WKWebView en macOS). Sin la etiqueta
// se compila el stub de main_nogui.go, de modo que `go build ./...` sigue siendo
// Go puro y la compilación cruzada del daemon/CLI no necesita estas dependencias.
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/daemon"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

//go:embed assets/icon.png
var iconPNG []byte

// Los iconos de bandeja van a 22px (tamaño nativo del panel): un pixmap grande
// reescalado por el host SNI se ve diminuto/borroso en GNOME.
//
//go:embed assets/tray-active.png
var trayActivePNG []byte

//go:embed assets/tray-paused.png
var trayPausedPNG []byte

//go:embed assets/tray-error.png
var trayErrorPNG []byte

func main() {
	var (
		configPath = flag.String("config", "", "ruta alternativa a config.toml")
		background = flag.Bool("background", false, "arrancar oculto en la bandeja (sin abrir ventana)")
		open       = flag.String("open", "", "abrir directamente un fichero versionado (ruta relativa del store)")
	)
	flag.Parse()

	cfg, err := config.LoadOrCreate(*configPath, false)
	if err != nil {
		log.Fatalf("unmess-app: cargando configuración: %v", err)
	}

	// Aseguramos un daemon en marcha y obtenemos un cliente de su API local.
	client, spawned, err := ensureDaemon(cfg.UI.Port)
	if err != nil {
		log.Fatalf("unmess-app: %v", err)
	}

	a := newAppState(client, spawned, *background)
	a.openTarget = *open
	if err := a.run(); err != nil {
		log.Fatalf("unmess-app: %v", err)
	}
}

// appState agrupa las piezas vivas de la app nativa.
type appState struct {
	client  *daemonClient
	app     *application.App
	window  *application.WebviewWindow
	tray    *application.SystemTray
	notif   *notifications.NotificationService
	pauseMI *application.MenuItem

	background bool
	openTarget string // ruta relativa a abrir al arrancar (vacío = inicio)
	quitting   atomic.Bool
	curPaused  atomic.Bool // último estado de pausa conocido (para el icono de bandeja)
	// stopSpawned detiene el daemon que arrancó esta app al salir (nil si el
	// daemon ya estaba en marcha por su cuenta: en ese caso no lo tocamos).
	stopSpawned func()
}

// urlFor construye la URL de la ventana para una ruta relativa del store. Sin
// ruta, la raíz; con ruta, el enrutado hash que la UI ya entiende.
func (a *appState) urlFor(rel string) string {
	if rel == "" {
		return a.client.baseURL + "/"
	}
	return a.client.baseURL + "/#/file/" + rel
}

func newAppState(client *daemonClient, spawned *exec.Cmd, background bool) *appState {
	a := &appState{client: client, background: background}
	if spawned != nil {
		a.stopSpawned = func() { _ = stopDaemon(spawned) }
	}
	return a
}

// notifSeq numera las notificaciones para darles ID único.
var notifSeq atomic.Uint64

func nextNotifID() uint64 { return notifSeq.Add(1) }

func (a *appState) run() error {
	a.notif = notifications.New()

	a.app = application.New(application.Options{
		Name:        "unmess",
		Description: "Protección automática y versionado de tus archivos",
		Icon:        iconPNG,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "ai.unmess.app",
			// Si se lanza una segunda instancia (p.ej. `unmess ui`), la primera
			// trae su ventana al frente en vez de abrir otra.
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				// Una segunda invocación (p.ej. `unmess ui <ruta>`) trae la
				// ventana al frente y, si trae un objetivo, navega a él.
				if rel := openArgFrom(data.Args); rel != "" && a.window != nil {
					a.window.SetURL(a.urlFor(rel))
				}
				a.showWindow()
			},
		},
		Services: []application.Service{
			application.NewService(a.notif),
		},
	})

	a.window = a.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "unmess",
		Width:            1180,
		Height:           780,
		MinWidth:         760,
		MinHeight:        480,
		URL:              a.urlFor(a.openTarget),
		Hidden:           a.background,
		BackgroundColour: application.RGBA{Red: 0xf6, Green: 0xf7, Blue: 0xf9, Alpha: 0xff},
	})

	// Cerrar la ventana la oculta a la bandeja en vez de terminar la app; solo
	// "Salir" en el menú de bandeja termina de verdad.
	a.window.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if a.quitting.Load() {
			return
		}
		e.Cancel()
		a.window.Hide()
	})

	a.buildTray()

	// Consumidor de eventos del daemon → notificaciones nativas.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.client.streamEvents(ctx, a.onDaemonEvent)

	// Estado inicial de pausa reflejado en el menú.
	if paused, err := a.client.status(); err == nil {
		a.setPauseMenuState(paused)
	}

	return a.app.Run()
}

// buildTray crea el icono de bandeja y su menú contextual.
func (a *appState) buildTray() {
	a.tray = a.app.SystemTray.New()
	a.tray.SetIcon(trayActivePNG)
	a.tray.SetTooltip("unmess — protección activa")

	menu := a.app.NewMenu()
	menu.Add("Abrir unmess").OnClick(func(_ *application.Context) {
		a.showWindow()
	})
	menu.AddSeparator()
	a.pauseMI = menu.AddCheckbox("Pausar la protección", true)
	a.pauseMI.OnClick(func(ctx *application.Context) {
		a.onPauseClicked(a.pauseMI.Checked())
	})
	menu.AddSeparator()
	menu.Add("Salir").OnClick(func(_ *application.Context) {
		a.quit()
	})

	a.tray.SetMenu(menu)
	// Clic en el icono muestra/oculta la ventana.
	a.tray.OnClick(func() { a.tray.ToggleWindow() })
	a.tray.AttachWindow(a.window)
}

// showWindow trae la ventana al frente.
func (a *appState) showWindow() {
	if a.window == nil {
		return
	}
	a.window.Show()
	a.window.Focus()
}

// onPauseClicked reacciona al toggle del menú: aplica el estado en el daemon.
func (a *appState) onPauseClicked(paused bool) {
	if err := a.client.setPaused(paused); err != nil {
		a.sendNotification("unmess", "No se pudo cambiar el estado: "+err.Error())
		// Revertimos el check visual si falló.
		a.setPauseMenuState(!paused)
		return
	}
	a.applyTrayState(paused)
}

// setPauseMenuState fija el check y el icono/tooltip de bandeja sin llamar al
// daemon (usado al reflejar cambios que llegan por el stream de eventos).
func (a *appState) setPauseMenuState(paused bool) {
	if a.pauseMI != nil {
		a.pauseMI.SetChecked(paused)
	}
	a.applyTrayState(paused)
}

// applyTrayState pone el icono y el tooltip de bandeja según el estado de pausa.
func (a *appState) applyTrayState(paused bool) {
	a.curPaused.Store(paused)
	if a.tray == nil {
		return
	}
	if paused {
		a.tray.SetIcon(trayPausedPNG)
		a.tray.SetTooltip("unmess — protección EN PAUSA")
	} else {
		a.tray.SetIcon(trayActivePNG)
		a.tray.SetTooltip("unmess — protección activa")
	}
}

// flashTrayError muestra brevemente el icono de error en la bandeja y luego
// vuelve al estado actual (activo/pausa).
func (a *appState) flashTrayError() {
	if a.tray == nil {
		return
	}
	a.tray.SetIcon(trayErrorPNG)
	a.tray.SetTooltip("unmess — error")
	time.AfterFunc(5*time.Second, func() { a.applyTrayState(a.curPaused.Load()) })
}

// onDaemonEvent traduce los eventos del daemon a notificaciones del SO y
// mantiene el menú en sincronía.
func (a *appState) onDaemonEvent(ev daemon.Event) {
	switch ev.Type {
	case daemon.EventVersioned:
		a.sendNotification("Nueva versión guardada", ev.Path)
	case daemon.EventRestored:
		a.sendNotification("Restauración completada", ev.Path)
	case daemon.EventError:
		a.sendNotification("unmess: error", ev.Message)
		a.flashTrayError()
	case daemon.EventPaused:
		a.setPauseMenuState(true)
	case daemon.EventResumed:
		a.setPauseMenuState(false)
	case daemon.EventPruned:
		// Silencioso salvo mensaje informativo en tooltip; no molestamos con toast.
	}
}

// sendNotification envía una notificación nativa; ignora errores (una
// notificación fallida no debe tumbar la app).
func (a *appState) sendNotification(title, body string) {
	if a.notif == nil {
		return
	}
	_ = a.notif.SendNotification(notifications.NotificationOptions{
		ID:    fmt.Sprintf("unmess-%d", nextNotifID()),
		Title: title,
		Body:  body,
	})
}

// quit termina la app de verdad y, si esta app arrancó el daemon, lo detiene.
func (a *appState) quit() {
	a.quitting.Store(true)
	if a.stopSpawned != nil {
		a.stopSpawned()
	}
	if a.app != nil {
		a.app.Quit()
	}
}
