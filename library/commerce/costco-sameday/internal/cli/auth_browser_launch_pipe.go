// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
//go:build !windows

// Launches the login browser with --remote-debugging-pipe: Chrome reads CDP
// commands on fd 3 and writes replies on fd 4, so no DevTools TCP port (and
// no DevToolsActivePort file) exists for other local users to reach.
// Preserved across regenerate via .printing-press-patches.

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// browserLoginTransport names the CDP transport for docs and tests.
const browserLoginTransport = "pipe"

// startLoginBrowser starts the browser and returns its CDP connection and a
// channel that receives the process exit (Wait is already running).
func startLoginBrowser(_ context.Context, bin string, args []string) (*exec.Cmd, cdpCaller, <-chan error, error) {
	// childR/parentW: commands to Chrome (its fd 3).
	// parentR/childW: replies from Chrome (its fd 4).
	childR, parentW, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating the DevTools pipe: %w", err)
	}
	parentR, childW, err := os.Pipe()
	if err != nil {
		childR.Close()
		parentW.Close()
		return nil, nil, nil, fmt.Errorf("creating the DevTools pipe: %w", err)
	}
	full := append([]string{"--remote-debugging-pipe"}, args...)
	proc := exec.Command(bin, full...) // #nosec G204 -- user-selected local browser binary.
	// nil = the null device: no copy goroutine, so Wait returns as soon as
	// the browser exits even if a child still holds an inherited handle.
	proc.Stdout, proc.Stderr = nil, nil
	proc.ExtraFiles = []*os.File{childR, childW} // fd 3, fd 4
	err = proc.Start()
	// Chrome holds its own copies now (or never started).
	childR.Close()
	childW.Close()
	if err != nil {
		parentR.Close()
		parentW.Close()
		return nil, nil, nil, fmt.Errorf("starting browser: %w", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- proc.Wait() }()
	return proc, newPipeCDP(parentR, parentW), exited, nil
}
