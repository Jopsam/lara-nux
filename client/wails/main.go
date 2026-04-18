package main

import (
	"embed"
	"log"

	"github.com/jopsam/lara-nux/client/wails/bridge"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var frontendAssets embed.FS

func main() {
	client := bridge.NewClient(defaultSocketPath())
	app := NewApp(client, newTrayManager())

	err := wails.Run(&options.App{
		Title:             "Lara Nux",
		Width:             1240,
		Height:            820,
		MinWidth:          980,
		MinHeight:         680,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: frontendAssets,
		},
		Menu:       app.menu(),
		OnStartup:  app.startup,
		OnDomReady: app.domReady,
		OnShutdown: app.shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "com.github.jopsam.lara-nux",
			OnSecondInstanceLaunch: app.onSecondInstanceLaunch,
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
