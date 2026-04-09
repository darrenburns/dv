package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

func openURLInBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser failed: %w", err)
	}
	return cmd.Process.Release()
}
