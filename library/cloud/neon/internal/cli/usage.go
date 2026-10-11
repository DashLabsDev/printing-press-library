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

func newNovelUsageCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Work with usage",
		Example: strings.Trim(`
  neon-pp-cli usage top --metric compute --since 7d --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2", "pp:parent-group": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelUsageTopCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelUsageTrendCmd(flags))
	return cmd
}
