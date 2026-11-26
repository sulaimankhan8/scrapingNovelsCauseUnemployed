package main

import (
	"embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"novelarchiver/internal/manga"
	"novelarchiver/internal/reader"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	app := NewApp()

	// The offline reader is served in-process at /library/... — no ports.
	var libHandler http.Handler = http.NotFoundHandler()
	if st := app.svc.Store(); st != nil {
		libHandler = &reader.Handler{Store: st, Translate: app.svc.TranslateChapter, Paused: app.svc.TranslatePause}
	}
	var mangaHandler http.Handler = http.NotFoundHandler()
	if st := app.manga.store(); st != nil {
		mangaHandler = &manga.Handler{Store: st}
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, manga.FilePrefix) {
			mangaHandler.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/library/") {
			libHandler.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})

	err := wails.Run(&options.App{
		Title:     "InkShelfs",
		Width:     1280,
		Height:    840,
		MinWidth:  360,
		MinHeight: 520,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: handler,
		},
		BackgroundColour: &options.RGBA{R: 14, G: 18, B: 22, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []interface{}{app, app.manga},
		Windows: &windows.Options{
			WebviewUserDataPath: filepath.Join(os.Getenv("LOCALAPPDATA"), "NovelArchiver", "WebView2"),
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
