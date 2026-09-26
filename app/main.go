package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/max-tsx/max-vpn/internal/tunnel"
)

//go:embed all:frontend/dist
var assets embed.FS

var _ = tunnel.Tunnel{}

func main() {
	svc := &VPN{}

	app := application.New(application.Options{
		Name:        "max-vpn",
		Description: "WireGuard VPN client",
		Services:    []application.Service{application.NewService(svc)},
		Icon:        trayIcon(false),
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyRegular,
		},
	})

	mainWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "main",
		Title:     "max-vpn",
		Width:     900,
		Height:    600,
		MinWidth:  900,
		MinHeight: 600,
		URL:       "/",
		Mac: application.MacWindow{
			Backdrop:                application.MacBackdropNormal,
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 38,
		},
	})

	mainWin.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		mainWin.Hide()
		e.Cancel()
	})
	showMain := func() { mainWin.Show(); mainWin.Focus() }
	app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) { showMain() })

	menu := application.NewMenu()
	menu.Add("Open max-vpn").OnClick(func(*application.Context) { showMain() })
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) { app.Quit() })

	tray := app.SystemTray.New()
	tray.SetTemplateIcon(trayIcon(false))
	tray.SetTooltip("max-vpn")
	tray.SetMenu(menu)

	svc.emit = func(name string, data ...any) { app.Event.Emit(name, data...) }

	app.Event.On("status", func(e *application.CustomEvent) {
		connected := false
		if m, ok := e.Data.(map[string]any); ok {
			connected = m["state"] == "connected"
		}
		tray.SetTemplateIcon(trayIcon(connected))
	})

	go svc.retryConnect()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
