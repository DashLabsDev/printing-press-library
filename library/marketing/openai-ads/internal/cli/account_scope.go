package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
)

// accountScopeSuffix returns a short, non-reversible discriminator derived from
// the configured Ads credential, or "" when no credential is resolvable.
//
// Each OpenAI Ads API key is scoped to exactly one ad account (see
// https://developers.openai.com/ads/api-reference/authentication). Without a
// per-credential local store, pointing the CLI at a second ad account would
// read, mix, and extend the previous account's campaigns, ads, and snapshot
// history under the same data.db. Deriving the filename from the credential
// keeps each account's mirror and history isolated.
//
// The suffix is a truncated SHA-256 of the credential: it never reveals the
// key, and it is stable for as long as that key is in use. Rotating a key
// starts a fresh mirror, which is the conservative outcome — a stale mirror is
// re-syncable, whereas cross-account contamination is not detectable after the
// fact.
func accountScopeSuffix() string {
	cred := strings.TrimSpace(os.Getenv("OPENAI_ADS_API_KEY"))
	if cred == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(cred))
	return hex.EncodeToString(sum[:])[:12]
}

// scopedDBFilename returns the account-scoped SQLite filename.
func scopedDBFilename() string {
	if suffix := accountScopeSuffix(); suffix != "" {
		return "data-" + suffix + ".db"
	}
	return "data.db"
}
