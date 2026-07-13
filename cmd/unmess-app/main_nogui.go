//go:build !gui

// Stub de unmess-app para builds sin la etiqueta `gui`. La app nativa real
// (bandeja + ventana WebView + notificaciones) vive en main_gui.go y requiere
// CGO y las librerías de WebView del SO. Este stub permite que `go build ./...`
// y la compilación cruzada del resto del proyecto (Go puro) sigan funcionando
// sin esas dependencias.
//
// Para compilar la app real:
//
//	go build -tags gui ./cmd/unmess-app
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr,
		"unmess-app se compiló sin soporte de GUI.\n"+
			"Recompila con la etiqueta de build: go build -tags gui ./cmd/unmess-app")
	os.Exit(1)
}
