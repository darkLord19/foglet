//go:build desktop || bindings

package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/darkLord19/foglet/internal/daemon"
	"github.com/darkLord19/foglet/internal/env"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var version = "dev"

//go:embed frontend/*
var assets embed.FS

func main() {
	if err := runDesktop(); err != nil {
		panic(err)
	}
}

func runDesktop() error {
	baseURL := "http://127.0.0.1:8080"
	if !isBindingsBuild {
		fogHome, err := env.FogHome()
		if err != nil {
			return err
		}
		const port = 8080
		baseURL, err = daemon.EnsureRunning(fogHome, port, 20*time.Second)
		if err != nil {
			return fmt.Errorf("ensure fogd running: %w", err)
		}
	}
	frontendFS, err := fs.Sub(assets, "frontend")
	if err != nil {
		return fmt.Errorf("load desktop assets: %w", err)
	}

	app := newDesktopApp(baseURL, version)
	return wails.Run(&options.App{
		Title:             "Fog Desktop",
		Width:             1460,
		Height:            920,
		MinWidth:          1160,
		MinHeight:         760,
		DisableResize:     false,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		AssetServer: &assetserver.Options{
			Assets: frontendFS,
		},
		OnStartup: app.startup,
		Bind: []interface{}{
			app,
		},
	})
}

type desktopApp struct {
	ctx        context.Context
	apiBaseURL string
	version    string
}

func newDesktopApp(apiBaseURL, version string) *desktopApp {
	return &desktopApp{
		apiBaseURL: apiBaseURL,
		version:    version,
	}
}

func (a *desktopApp) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *desktopApp) APIBaseURL() string {
	return a.apiBaseURL
}

func (a *desktopApp) Version() string {
	return a.version
}

func (a *desktopApp) OpenExternal(rawURL string) {
	if a.ctx == nil {
		return
	}
	runtime.BrowserOpenURL(a.ctx, rawURL)
}
