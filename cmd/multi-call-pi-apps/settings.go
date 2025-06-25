package main

import (
	"fmt"
	"os"

	"github.com/botspot/pi-apps/pkg/settings"
)

func runSettings() {
	if err := settings.Main(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
