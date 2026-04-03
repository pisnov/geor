package main

import (
	"log"
	"os"
	"yt-audio/player"
	"yt-audio/ui"
)

func main() {
	// Redirect all log output to file — keeps TUI terminal completely clean
	if f, err := os.OpenFile("/tmp/yt-audio.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		log.SetOutput(f)
		defer f.Close()
	}

	p := player.NewPlayer()
	ui.StartTUI(p)
}
