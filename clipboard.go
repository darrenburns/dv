package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// Prefer a native helper so success means the clipboard was actually written.
// Remote terminal sessions use OSC 52, whose delivery cannot be confirmed.
func copyToClipboard(text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var commands [][]string
	if os.Getenv("SSH_CONNECTION") == "" && os.Getenv("SSH_TTY") == "" {
		switch runtime.GOOS {
		case "darwin":
			commands = [][]string{{"pbcopy"}}
		case "windows":
			commands = [][]string{{"clip.exe"}}
		default:
			if os.Getenv("WAYLAND_DISPLAY") != "" {
				commands = append(commands, []string{"wl-copy"})
			}
			if os.Getenv("DISPLAY") != "" {
				commands = append(commands, []string{"xclip", "-selection", "clipboard"}, []string{"xsel", "--clipboard", "--input"})
			}
		}
	}
	for _, args := range commands {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	if _, err := os.Stdout.WriteString(ansi.SetClipboard(uv.SystemClipboard, text)); err != nil {
		return err
	}
	return fmt.Errorf("clipboard requested, unconfirmed; aside review feedback can export it")
}
