// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
//go:build !windows

// Launches the login browser with --remote-debugging-pipe: Chrome reads CDP
// commands on fd 3 and writes replies on fd 4, so no DevTools TCP port (and
// no DevToolsActivePort file) exists for other local users to reach.
// Preserved across regenerate via .printing-press-patches.

package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// browserLoginTransport names the CDP transport for docs and tests.
const browserLoginTransport = "pipe"

func startLoginBrowser(bin string, args []string) (*exec.Cmd, cdpCaller, error) {
	// childR/parentW: commands to Chrome (its fd 3).
	// parentR/childW: replies from Chrome (its fd 4).
	childR, parentW, err := os.Pipe()
	if err != nil {
		return nil, nil, fmt.Errorf("creating the DevTools pipe: %w", err)
	}
	parentR, childW, err := os.Pipe()
	if err != nil {
		childR.Close()
		parentW.Close()
		return nil, nil, fmt.Errorf("creating the DevTools pipe: %w", err)
	}
	full := append([]string{"--remote-debugging-pipe"}, args...)
	proc := exec.Command(bin, full...) // #nosec G204 -- user-selected local browser binary.
	proc.Stdout, proc.Stderr = io.Discard, io.Discard
	proc.ExtraFiles = []*os.File{childR, childW} // fd 3, fd 4
	err = proc.Start()
	// Chrome holds its own copies now (or never started).
	childR.Close()
	childW.Close()
	if err != nil {
		parentR.Close()
		parentW.Close()
		return nil, nil, fmt.Errorf("starting browser: %w", err)
	}
	return proc, newPipeCDP(parentR, parentW), nil
}
