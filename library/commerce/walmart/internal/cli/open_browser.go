// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openInDefaultBrowser opens url in the user's default browser via the OS
// handler (open / xdg-open). It never drives Chrome DevTools / CDP.
func openInDefaultBrowser(rawURL string) error {
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
		return fmt.Errorf("open browser: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}
