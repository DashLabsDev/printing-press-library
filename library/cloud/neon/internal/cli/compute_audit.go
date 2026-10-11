// pp:data-source local
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/neon/internal/store"
)

type computeFinding struct {
	Project  string `json:"project"`
	Branch   string `json:"branch"`
	Endpoint string `json:"endpoint"`
	Issue    string `json:"issue"`
	Detail   string `json:"detail"`
}

func auditNeonCompute(db *store.Store, maxCU float64) ([]computeFinding, error) {
	result := make([]computeFinding, 0)
	branches, err := neonRows(db, "branches", "projects/branches", "projects_branches")
	if err != nil {
		return nil, err
	}
	defaults := map[string]bool{}
	known := map[string]bool{}
	for _, b := range branches {
		key := projectOf(b) + "/" + ns(b, "id")
		defaults[key] = nb(b, "default")
		known[key] = true
	}
	endpoints, err := neonRows(db, "endpoints", "projects/endpoints", "projects_endpoints")
	if err != nil {
		return nil, err
	}
	for _, e := range endpoints {
		p, b, id := projectOf(e), ns(e, "branch_id"), ns(e, "id")
		add := func(issue, detail string) { result = append(result, computeFinding{p, b, id, issue, detail}) }
		if v, ok := e["suspend_timeout_seconds"].(float64); ok && v == -1 {
			add("suspend_disabled", "suspend_timeout_seconds=-1")
		}
		min, minOK := e["autoscaling_limit_min_cu"].(float64)
		max, maxOK := e["autoscaling_limit_max_cu"].(float64)
		if minOK && maxOK && min == max {
			add("autoscaling_fixed", fmt.Sprintf("min=max=%g CU", max))
		}
		if maxOK && max > maxCU && known[p+"/"+b] && !defaults[p+"/"+b] {
			add("non_default_large", fmt.Sprintf("max=%g CU exceeds %g CU", max, maxCU))
		}
	}
	return result, nil
}
func newNovelComputeAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxCU float64
	cmd := &cobra.Command{Use: "audit", Short: "Find compute endpoints with costly configuration.", Example: strings.Trim(`
  neon-pp-cli compute audit --max-cu 2 --db ./dev.db --json
`, "\n"), Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true", "pp:happy-args": "--max-cu=2"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "compute audit")
		}
		if len(args) != 0 {
			return neonMissing(cmd, "unexpected positional arguments")
		}
		if maxCU <= 0 {
			return neonMissing(cmd, "--max-cu must be positive")
		}
		db, err := openNeonMirror(cmd, flags, dbPath, "endpoints")
		if err != nil {
			return err
		}
		if db != nil {
			defer db.Close()
		}
		rows, err := auditNeonCompute(db, maxCU)
		if err != nil {
			return err
		}
		return neonOutput(cmd, flags, rows)
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path")
	cmd.Flags().Float64Var(&maxCU, "max-cu", 2, "Maximum CU allowed on non-default branches")
	return cmd
}
