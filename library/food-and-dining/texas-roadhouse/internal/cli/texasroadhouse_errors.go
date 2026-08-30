// Copyright 2026 Thomas McCormick and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(waitlist-pii-stdin-and-here-checkin): Cloudflare-challenge 403 + no fake list command.

package cli

import (
	"fmt"
	"strings"
)

const cloudflareChallengeHint = "Cloudflare challenge. Naked HTTP gets HTTP 403; this CLI uses Chrome-compatible transport (live-tested late Aug 2026). If this persists, the origin returned a challenge page (cf-mitigated / Just a moment) instead of the waitlist API. Run 'texas-roadhouse-pp-cli doctor'."

const notFoundStoresHint = "resource not found. This CLI has no list command; look up stores with 'stores --lat <lat> --long <long>' and quotes with 'texasroadhouse get-quote <extref>'."

// LooksLikeCloudflareChallenge reports a Cloudflare interstitial or
// cf-mitigated 403, as opposed to a generic permission denial.
func LooksLikeCloudflareChallenge(msg string) bool {
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "403") {
		return false
	}
	return strings.Contains(lower, "cloudflare") ||
		strings.Contains(lower, "cf-mitigated") ||
		strings.Contains(lower, "just a moment") ||
		strings.Contains(lower, "challenges.cloudflare.com")
}

func classifyWaitlistHTTP403(err error) error {
	if err == nil {
		return nil
	}
	if LooksLikeCloudflareChallenge(err.Error()) {
		return authErr(fmt.Errorf("%w\nhint: %s", err, cloudflareChallengeHint))
	}
	return authErr(fmt.Errorf("%w\nhint: permission denied. This API is configured without credentials, so the service may be blocking the request by rate limit, geography, bot protection, or endpoint policy."+
		"\n      Run 'texas-roadhouse-pp-cli doctor' to check connectivity.", err))
}

func classifyWaitlistHTTP404(err error) error {
	return notFoundErr(fmt.Errorf("%w\nhint: %s", err, notFoundStoresHint))
}

// CloudflareChallengeToolMessage is the MCP-facing form of the same 403 hint.
func CloudflareChallengeToolMessage(msg string) string {
	return "Cloudflare challenge: " + msg + "\nhint: " + cloudflareChallengeHint
}
