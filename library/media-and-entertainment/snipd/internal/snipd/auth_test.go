// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored test — NOT generator output.
package snipd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSignInURLCarriesUUID(t *testing.T) {
	got := SignInURL("cc437cd9-072d-43c9-b04e-808d88bfc907")
	want := "https://app.snipd.com/obsidian/auth?uuid=cc437cd9-072d-43c9-b04e-808d88bfc907"
	if got != want {
		t.Errorf("SignInURL() = %q, want %q", got, want)
	}
}

func TestPollForTokenSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("uuid"); got != "pair-me" {
			t.Errorf("uuid query = %q, want pair-me", got)
		}
		if r.URL.Path != "/obsidian/auth" {
			t.Errorf("path = %q, want /obsidian/auth", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"token":"tok-abc123"}`))
	}))
	defer srv.Close()

	got, err := PollForToken(context.Background(), srv.URL, "pair-me")
	if err != nil {
		t.Fatalf("PollForToken() error = %v", err)
	}
	if got != "tok-abc123" {
		t.Errorf("token = %q, want tok-abc123", got)
	}
}

// The server closes the long-poll early (proxy reap) before the user has
// finished signing in. The retry should be transparent.
func TestPollForTokenRetriesEmptyResponse(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`{"token":"late-token"}`))
	}))
	defer srv.Close()

	got, err := PollForToken(context.Background(), srv.URL, "pair-me")
	if err != nil {
		t.Fatalf("PollForToken() error = %v", err)
	}
	if got != "late-token" {
		t.Errorf("token = %q, want late-token", got)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestPollForTokenGivesUpAfterAttempts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	_, err := PollForToken(context.Background(), srv.URL, "pair-me")
	if err != ErrSignInIncomplete {
		t.Errorf("error = %v, want ErrSignInIncomplete", err)
	}
}

// A success-shaped body holds the token, so no error string may echo the body.
func TestPollForTokenErrorNeverLeaksBody(t *testing.T) {
	const secret = "super-secret-token-value"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"token":"` + secret + `"}`))
	}))
	defer srv.Close()

	_, err := PollForToken(context.Background(), srv.URL, "pair-me")
	if err == nil {
		t.Fatal("expected an error on HTTP 500")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error leaked the response body: %v", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should name the status code, got: %v", err)
	}
}

func TestPollForTokenRespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := PollForToken(ctx, srv.URL, "pair-me")
	if err == nil {
		t.Fatal("expected a context error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("took %v — context deadline was not honoured", elapsed)
	}
}
