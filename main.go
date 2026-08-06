// Command hashdupes is the Wails desktop application entry point. It wires the
// SQLite-backed index, the versioned v1 API, and a Wails runtime "bridge" that
// satisfies the API's Emitter and DirPicker interfaces (keeping the API package
// itself free of any Wails dependency).
package main

import (
	"context"
	"embed"
	"log"

	v1 "hashdupes/internal/api/v1"
	"hashdupes/internal/appdir"
	"hashdupes/internal/index"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// app holds process-wide state and the Wails context (set at startup).
type app struct {
	ctx   context.Context
	store *index.Store
}

func (a *app) startup(ctx context.Context) { a.ctx = ctx }

func (a *app) shutdown(context.Context) {
	if a.store != nil {
		_ = a.store.Close()
	}
}

// bridge adapts the Wails runtime to the v1.Emitter and v1.DirPicker
// interfaces. It reads the app context lazily so it is valid after startup.
type bridge struct{ app *app }

func (b bridge) Emit(event string, data ...any) {
	wruntime.EventsEmit(b.app.ctx, event, data...)
}

func (b bridge) PickDirectory(title string) (string, error) {
	return wruntime.OpenDirectoryDialog(b.app.ctx, wruntime.OpenDialogOptions{Title: title})
}

func main() {
	a := &app{}

	path, err := appdir.DefaultDBPath()
	if err != nil {
		log.Fatalf("resolve db path: %v", err)
	}
	store, err := index.Open(context.Background(), path)
	if err != nil {
		log.Fatalf("open index: %v", err)
	}
	a.store = store

	br := bridge{app: a}
	api := v1.New(store, br, br)

	err = wails.Run(&options.App{
		Title:     "hashdupes",
		Width:     1200,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		// Match the frontend's dark theme background to avoid a white flash on launch.
		BackgroundColour: &options.RGBA{R: 2, G: 8, B: 23, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  a.startup,
		OnShutdown: a.shutdown,
		Bind: []any{
			api,
		},
	})
	if err != nil {
		log.Fatalf("wails: %v", err)
	}
}
