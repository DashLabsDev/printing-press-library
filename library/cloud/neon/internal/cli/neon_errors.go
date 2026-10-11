// Hand-authored: Neon-specific API error hints (hooked from classifyAPIError).

package cli

import (
	"fmt"
	"strings"
)

// neonAPIErrorHint replaces the generic "resource not found" hint for Neon's
// credential- and plan-gated refusals, which Neon returns as 404/403.
func neonAPIErrorHint(err error, msg string) error {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "not allowed for organization api keys"):
		return notFoundErr(fmt.Errorf("%w\nhint: feature not enabled for organization API keys. Use a personal API key (Account settings > API keys) for users, regions and api-keys commands", err))
	case strings.Contains(lower, "not available") && strings.Contains(lower, "plans and above"):
		return notFoundErr(fmt.Errorf("%w\nhint: your Neon plan does not include this endpoint (Launch/Scale plans and above); usage top/trend fall back to sync snapshots", err))
	}
	return nil
}
