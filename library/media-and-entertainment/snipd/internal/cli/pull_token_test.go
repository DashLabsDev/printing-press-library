// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored test — NOT generator output.
package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/snipd/internal/config"
)

// The hand-built Snipd export layer must accept a credential from either place
// the rest of the CLI is willing to store one. Before this resolver existed,
// pull read cfg.SnipdToken directly, so `auth set-token` (which persists into
// AccessToken via the generated SaveTokens) left `doctor` reporting "Auth:
// configured" while every pull failed with "no SNIPD_TOKEN set".
func TestResolveSnipdToken(t *testing.T) {
	tests := []struct {
		name       string
		snipdToken string
		accessTok  string
		want       string
	}{
		{
			name:       "env var only",
			snipdToken: "env-token",
			want:       "env-token",
		},
		{
			// The regression case: this is exactly what `auth set-token` writes.
			name:      "credentials file only (auth set-token)",
			accessTok: "saved-token",
			want:      "saved-token",
		},
		{
			// Mirrors config.AuthHeader(): env beats file.
			name:       "both set, env wins",
			snipdToken: "env-token",
			accessTok:  "saved-token",
			want:       "env-token",
		},
		{
			name: "neither set",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{SnipdToken: tt.snipdToken, AccessToken: tt.accessTok}
			if got := resolveSnipdToken(cfg); got != tt.want {
				t.Errorf("resolveSnipdToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveSnipdTokenNilConfig(t *testing.T) {
	if got := resolveSnipdToken(nil); got != "" {
		t.Errorf("resolveSnipdToken(nil) = %q, want empty", got)
	}
}
