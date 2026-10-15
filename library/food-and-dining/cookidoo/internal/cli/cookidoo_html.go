// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0.
// Hand-authored extension kept outside generated command files.

package cli

import "encoding/json"

// ExtractCookidooRecipeHTML normalizes a public Cookidoo HTML page to its
// embedded schema.org JSON-LD object for typed and mirrored MCP responses.
func ExtractCookidooRecipeHTML(raw []byte, baseURL, contentType string) (json.RawMessage, error) {
	return extractHTMLResponse(raw, htmlExtractionOptions{
		Mode:           "embedded-json",
		BaseURL:        baseURL,
		ContentType:    contentType,
		ScriptSelector: `script[type="application/ld+json"]`,
	})
}
