// Command unmessd es el daemon de unmessai: vigila las rutas configuradas,
// versiona los cambios al store y ejecuta la poda periódica. El servidor de API
// HTTP se montará más adelante sobre el enganche daemon.Options.Hook.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/luisobz/unmess-ai/internal/api"
	"github.com/luisobz/unmess-ai/internal/config"
	"github.com/luisobz/unmess-ai/internal/daemon"
)

// version es la versión del binario; se expone en GET /api/status.
const version = "0.1.0-dev"

func main() {
	configPath := flag.String("config", "", "ruta alternativa a config.toml")
	flag.Parse()

	logger := log.New(os.Stderr, "unmessd ", log.LstdFlags)

	cfg, err := config.LoadOrCreate(*configPath)
	if err != nil {
		logger.Fatalf("cargando configuración: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	opts := daemon.Options{
		Logger: logger,
		// Hook monta el servidor HTTP local (API + UI) sobre el Runtime ya
		// inicializado. Un fallo al abrir el puerto no tumba el daemon.
		Hook: func(ctx context.Context, rt *daemon.Runtime) error {
			return api.Serve(ctx, rt, version)
		},
	}
	if err := daemon.Run(ctx, cfg, opts); err != nil {
		logger.Fatalf("daemon: %v", err)
	}
}
