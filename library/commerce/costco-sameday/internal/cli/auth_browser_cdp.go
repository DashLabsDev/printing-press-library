// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Minimal Chrome DevTools Protocol client for the one-step `auth login`.
// It speaks CDP over Chrome's --remote-debugging-pipe (NUL-delimited JSON on
// the browser's fds 3 and 4), so no DevTools TCP port is opened and other
// local users cannot connect to the login browser. Only three browser-level
// methods are ever sent: Browser.getVersion, Target.getTargets,
// Storage.getCookies (plus Browser.close). Events are ignored.
// Preserved across regenerate via .printing-press-patches.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

// cdpCaller sends one CDP command to the browser target and decodes the
// result into res (nil to discard).
type cdpCaller interface {
	Call(ctx context.Context, method string, params, res any) error
	Close() error
}

// cdpTargetInfo is the subset of Target.TargetInfo the login loop reads.
type cdpTargetInfo struct {
	Type  string `json:"type"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

// cdpCookie is the subset of Network.Cookie the login loop reads.
type cdpCookie struct {
	Name    string  `json:"name"`
	Value   string  `json:"value"`
	Domain  string  `json:"domain"`
	Expires float64 `json:"expires"`
	Session bool    `json:"session"`
}

type cdpRemoteError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *cdpRemoteError) Error() string { return fmt.Sprintf("cdp error %d: %s", e.Code, e.Message) }

type cdpReply struct {
	result json.RawMessage
	err    error
}

// errCDPClosed reports that the browser end of the pipe went away.
var errCDPClosed = errors.New("login browser connection closed")

// pipeCDP is a cdpCaller over a pair of pipes.
type pipeCDP struct {
	r io.ReadCloser
	w io.WriteCloser

	wmu sync.Mutex

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan cdpReply
	err     error
	done    chan struct{}
	once    sync.Once
}

// newPipeCDP starts reading replies from r; commands are written to w.
func newPipeCDP(r io.ReadCloser, w io.WriteCloser) *pipeCDP {
	p := &pipeCDP{r: r, w: w, pending: map[int64]chan cdpReply{}, done: make(chan struct{})}
	go p.readLoop()
	return p
}

func (p *pipeCDP) readLoop() {
	br := bufio.NewReaderSize(p.r, 1<<20)
	for {
		frame, err := br.ReadBytes(0)
		if err != nil {
			p.fail(errCDPClosed)
			return
		}
		frame = frame[:len(frame)-1]
		var msg struct {
			ID     int64           `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *cdpRemoteError `json:"error"`
		}
		if json.Unmarshal(frame, &msg) != nil || msg.ID == 0 {
			continue // event or noise
		}
		p.mu.Lock()
		ch := p.pending[msg.ID]
		delete(p.pending, msg.ID)
		p.mu.Unlock()
		if ch == nil {
			continue
		}
		if msg.Error != nil {
			ch <- cdpReply{err: msg.Error}
		} else {
			ch <- cdpReply{result: msg.Result}
		}
	}
}

func (p *pipeCDP) fail(err error) {
	p.once.Do(func() {
		p.mu.Lock()
		p.err = err
		p.mu.Unlock()
		close(p.done)
	})
}

// Call implements cdpCaller.
func (p *pipeCDP) Call(ctx context.Context, method string, params, res any) error {
	p.mu.Lock()
	if p.err != nil {
		err := p.err
		p.mu.Unlock()
		return err
	}
	p.nextID++
	id := p.nextID
	ch := make(chan cdpReply, 1)
	p.pending[id] = ch
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
	}()

	req := map[string]any{"id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	p.wmu.Lock()
	_, err = p.w.Write(append(b, 0))
	p.wmu.Unlock()
	if err != nil {
		p.fail(errCDPClosed)
		return errCDPClosed
	}
	select {
	case rep := <-ch:
		if rep.err != nil {
			return rep.err
		}
		if res == nil || len(rep.result) == 0 {
			return nil
		}
		return json.Unmarshal(rep.result, res)
	case <-p.done:
		return errCDPClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close implements cdpCaller.
func (p *pipeCDP) Close() error {
	p.fail(errCDPClosed)
	werr := p.w.Close()
	rerr := p.r.Close()
	return errors.Join(werr, rerr)
}
