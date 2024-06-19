package main

import (
	"os/exec"

	"golang.design/x/clipboard"
)

func CopyToClipboard(text string) error {
	// Attempt to use gclip.
	clipboard.Write(clipboard.FmtText, []byte(text))

	// For wayland support, we'll use wl-copy if available.
	// Just disregard errors from this command.
	cmd := exec.Command("wl-copy", text)
	cmd.Run()

	return nil
}
