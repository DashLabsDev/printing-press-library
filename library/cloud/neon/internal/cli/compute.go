// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored command group.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

func newNovelComputeCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "compute",
		Short: "Work with compute",
		Example: strings.Trim(`
  neon-pp-cli compute audit --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2", "pp:parent-group": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelComputeAuditCmd(flags))
	return cmd
}
