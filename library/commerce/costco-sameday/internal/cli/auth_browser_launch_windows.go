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
	"io"
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

func startLoginBrowser(bin string, args []string) (*exec.Cmd, cdpCaller, error) {
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
	proc.Stdout, proc.Stderr = io.Discard, io.Discard
	if err := proc.Start(); err != nil {
		return nil, nil, err
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(portFile); err == nil { // #nosec G304 -- CLI-owned profile dir.
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) >= 2 && strings.TrimSpace(lines[0]) != "" {
				ctx, cancel := context.WithCancel(context.Background())
				b, err := chromedp.NewBrowser(ctx, "ws://127.0.0.1:"+strings.TrimSpace(lines[0])+strings.TrimSpace(lines[1]))
				if err != nil {
					cancel()
					return proc, &deadCDP{}, nil
				}
				return proc, &wsCDP{b: b, cancel: cancel}, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return proc, &deadCDP{}, nil
}

// deadCDP fails every call; runBrowserLogin then reports the browser as
// closed or the profile as in use.
type deadCDP struct{}

func (deadCDP) Call(context.Context, string, any, any) error { return errCDPClosed }
func (deadCDP) Close() error                                 { return nil }
