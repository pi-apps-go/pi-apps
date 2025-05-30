package main

import (
	"log"
	"os"

	"github.com/botspot/pi-apps/pkg/gui"
	"github.com/gotk3/gotk3/gtk"
)

func main() {
	// Initialize GTK
	gtk.Init(nil)

	// Ensure PI_APPS_DIR is set
	if os.Getenv("PI_APPS_DIR") == "" {
		// Set it to the current working directory's parent
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal("Failed to get working directory:", err)
		}
		os.Setenv("PI_APPS_DIR", wd+"/..")
	}

	// Run the app browser demo
	if err := gui.RunAppBrowserDemo(); err != nil {
		log.Fatal("Error running demo:", err)
	}
}
