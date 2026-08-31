// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shim — NOT generator output.
//
// The 4.31.x chassis ships a writeDryRun helper in internal/cli/helpers.go for
// exactly this. This CLI is printed on 4.28.0, which has no such helper, and a
// --dry-run branch that prints prose — or returns nil and emits nothing at all —
// fails the live matrix's json_fidelity probe with "invalid JSON" under
// `--dry-run --json` (upstream cli-printing-press issue 4321, closed wontfix
// because the helper already exists upstream).
//
// Field names and the "would" phrasing match 4.31.x's dryRunResult exactly, so
// the chassis upgrade can delete this file and switch the call sites to the
// framework helper with no change in output.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

type dryRunResult struct {
	DryRun bool   `json:"dry_run"`
	Action string `json:"action"`
	Would  string `json:"would"`
	// URL is a shim-only addition: auth login's suppressed side effect is
	// opening a browser, and the sign-in URL is the one thing a human running
	// --dry-run actually wants. It carries no secret.
	URL string `json:"url,omitempty"`
}

// writeDryRunShim reports a suppressed side effect in whichever format the
// caller asked for. Mirrors 4.31.x's writeDryRun.
func writeDryRunShim(w io.Writer, flags *rootFlags, action string) error {
	would := "run " + action + "; no changes made"
	if flags != nil && flags.asJSON {
		return json.NewEncoder(w).Encode(dryRunResult{DryRun: true, Action: action, Would: would})
	}
	_, err := fmt.Fprintf(w, "dry-run: would %s\n", would)
	return err
}
