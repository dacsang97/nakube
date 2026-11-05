package main

import (
	"context"
	"log"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/desktop"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		runMCP()
		return
	}
	cluster.PreparePath()
	app := desktop.New(cluster.NewSession())
	app.RestoreAppearance()
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     "NaKube",
			Width:     1100,
			Height:    720,
			MinWidth:  800,
			MinHeight: 560,
			StateKey:  "main",
			Content:   ui.View(app.View),
		})
		app.Attach(win)
		go app.Bootstrap(context.Background())
		go app.StartPhoneListener(context.Background())
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
