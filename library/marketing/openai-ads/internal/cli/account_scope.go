package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/config"
)

// accountScopeSuffix returns a short, non-reversible discriminator derived from
// the *effective* Ads credential, or "" when no credential is resolvable.
//
// Each OpenAI Ads API key is scoped to exactly one ad account (see
// https://developers.openai.com/ads/api-reference/authentication). Without a
// per-credential local store, pointing the CLI at a second ad account would
// read, mix, and extend the previous account's campaigns, ads, and snapshot
// history under the same data.db.
//
// The credential is resolved through config.Load so that every supported auth
// path participates in scoping, not just the environment variable: a token
// persisted by 'auth set-token' into the credentials file authenticates API
// requests just as an env var does, and must therefore select the same
// account-scoped database. config.Load applies the documented precedence
// (credentials file first, environment override last), so this mirrors exactly
// what the API client will send.
//
// The suffix is a truncated SHA-256 of the credential: it never reveals the
// key, and it is stable for as long as that key is in use. Rotating a key
// starts a fresh mirror, which is the conservative outcome — a stale mirror is
// re-syncable with one 'sync', whereas cross-account contamination is not
// detectable after the fact.
func accountScopeSuffix() string {
	cred := effectiveAdsCredential()
	if cred == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(cred))
	return hex.EncodeToString(sum[:])[:12]
}

// effectiveAdsCredential returns the Ads API key the client would actually
// authenticate with, or "" when none is configured. Failures to load config
// are non-fatal: an unscoped database is the pre-existing behavior and is
// preferable to blocking every local command on a config parse error.
func effectiveAdsCredential() string {
	cfg, err := config.Load("")
	if err != nil || cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.OpenaiAdsApiKey)
}

// scopedDBFilename returns the account-scoped SQLite filename.
func scopedDBFilename() string {
	if suffix := accountScopeSuffix(); suffix != "" {
		return "data-" + suffix + ".db"
	}
	return "data.db"
}
