// Copyright 2026 H179922 and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/ofa-health-clearances/internal/config"
	"github.com/mvanhorn/printing-press-library/library/health/ofa-health-clearances/internal/platform"
)

// A 429 whose Retry-After outlasts --timeout must still surface as a typed
// rate-limit error (exit 7), not a bare deadline error (exit 5).
func TestRateLimitWaitTimeoutKeepsTypedError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL}, 300*time.Millisecond, 0)
	_, err := c.Get(context.Background(), "/api/as.php", nil)
	if err == nil {
		t.Fatal("expected an error from a persistently throttled endpoint")
	}
	var rl *platform.RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("want *platform.RateLimitedError after the wait timed out, got %T: %v", err, err)
	}
}
