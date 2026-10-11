// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
//go:build windows

// Windows fallback for the login browser: os/exec cannot hand Chrome the
// fd 3/4 pipe pair, so the browser listens on a DevTools port bound to
// 127.0.0.1 with a random port (--remote-debugging-port=0). Residual risk:
// another process running on the same machine could connect to that port
// for the minute or two the login window is open. The profile directory is
// private (0700) and the browser is closed as soon as the cookies are read.
// Preserved across regenerate via .printing-press-patches.

package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const browserLoginTransport = "port"

type wsCDP struct {
	b      *chromedp.Browser
	cancel context.CancelFunc
}

func (w *wsCDP) Call(ctx context.Context, method string, params, res any) error {
	return w.b.Execute(ctx, method, params, res)
}

func (w *wsCDP) Close() error { w.cancel(); return nil }

// startLoginBrowser starts the browser, waits for its DevTools port file
// until ctx ends or the browser exits, and returns the CDP connection and a
// channel that receives the process exit (Wait is already running).
func startLoginBrowser(ctx context.Context, bin string, args []string) (*exec.Cmd, cdpCaller, <-chan error, error) {
	profile := ""
	for _, a := range args {
		if strings.HasPrefix(a, "--user-data-dir=") {
			profile = strings.TrimPrefix(a, "--user-data-dir=")
		}
	}
	portFile := filepath.Join(profile, "DevToolsActivePort")
	_ = os.Remove(portFile)
	full := append([]string{"--remote-debugging-port=0", "--remote-debugging-address=127.0.0.1"}, args...)
	proc := exec.Command(bin, full...) // #nosec G204 -- user-selected local browser binary.
	// nil = the null device: no copy goroutine, so Wait returns as soon as
	// the browser exits even if a child still holds an inherited handle.
	proc.Stdout, proc.Stderr = nil, nil
	if err := proc.Start(); err != nil {
		return nil, nil, nil, err
	}
	waited := make(chan error, 1)
	go func() { waited <- proc.Wait() }()
	exited := make(chan error, 1)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if data, err := os.ReadFile(portFile); err == nil { // #nosec G304 -- CLI-owned profile dir.
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) >= 2 && strings.TrimSpace(lines[0]) != "" {
				go func() { exited <- <-waited }()
				bctx, cancel := context.WithCancel(context.Background())
				b, err := chromedp.NewBrowser(bctx, "ws://127.0.0.1:"+strings.TrimSpace(lines[0])+strings.TrimSpace(lines[1]))
				if err != nil {
					cancel()
					return proc, deadCDP{}, exited, nil
				}
				return proc, &wsCDP{b: b, cancel: cancel}, exited, nil
			}
		}
		select {
		case err := <-waited:
			// The browser exited before opening DevTools (profile in use or
			// closed); runBrowserLogin maps this from the exit channel.
			exited <- err
			return proc, deadCDP{}, exited, nil
		case <-ctx.Done():
			go func() { exited <- <-waited }()
			return proc, deadCDP{}, exited, nil
		case <-tick.C:
		}
	}
}

// deadCDP fails every call; runBrowserLogin then reports the browser as
// closed or the profile as in use.
type deadCDP struct{}

func (deadCDP) Call(context.Context, string, any, any) error { return errCDPClosed }
func (deadCDP) Close() error                                 { return nil }
