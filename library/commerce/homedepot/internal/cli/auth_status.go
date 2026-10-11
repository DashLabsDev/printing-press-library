package cli

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"time"
)

// pp:data-source local
func newNovelAuthStatusCmd(flags *rootFlags) *cobra.Command {
	var verify bool
	cmd := &cobra.Command{Use: "status", Short: "Inspect the session; --verify checks a signed-in request", Args: cobra.NoArgs, Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true", "pp:happy-args": "--verify"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "auth status")
		}
		path, err := sessionPath()
		if err != nil {
			return err
		}
		cookies, err := readHomeDepotCookies(path, true)
		result := map[string]any{"session_present": err == nil, "authenticated": false, "verified": false, "live_validated": false, "state": "missing"}
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return flags.printJSON(cmd, result)
			}
			result["state"] = "unreadable"
			_ = flags.printJSON(cmd, result)
			return authErr(fmt.Errorf("private session unreadable: %w", err))
		}
		result["cookie_count"] = len(cookies)
		if _, err = credentialsFromHomeDepotCookies(cookies); err != nil {
			result["state"] = "invalid_or_expired"
			_ = flags.printJSON(cmd, result)
			return err
		}
		result["authenticated"] = true
		result["state"] = "present_unverified"
		for _, ck := range cookies {
			if ck.Name == "THD_CUSTOMER" && !ck.Expires.IsZero() {
				result["expires_at"] = ck.Expires.UTC().Format(time.RFC3339)
			}
		}
		if verify {
			if err = verifyHomeDepotSession(cmd.Context(), flags, cookies); err != nil {
				result["state"] = "verification_failed"
				result["authenticated"] = false
				_ = flags.printJSON(cmd, result)
				return err
			}
			result["verified"] = true
			result["live_validated"] = true
			result["state"] = "verified"
		}
		return flags.printJSON(cmd, result)
	}}
	cmd.Flags().BoolVar(&verify, "verify", false, "Verify a harmless signed-in order-history request")
	return cmd
}
