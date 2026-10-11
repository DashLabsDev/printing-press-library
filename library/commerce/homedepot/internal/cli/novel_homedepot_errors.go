// Copyright 2026 Greg Mushen and contributors. Licensed under Apache-2.0.
package cli

import (
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/homedepot/internal/client"
)

// Provider errors can echo customer identifiers and request bodies. Preserve
// their exit classification without returning private paths or provider text.
func homeDepotRequestError(action string, status int, err error) error {
	var provider *client.APIError
	if errors.As(err, &provider) {
		status = provider.StatusCode
	}
	message := fmt.Errorf("%s failed; retry the read or check 'auth status --verify'", action)
	if status > 0 {
		message = fmt.Errorf("%s failed (HTTP %d); check 'auth status --verify'", action, status)
	}
	switch status {
	case 401, 403:
		return authErr(message)
	case 429:
		return rateLimitErr(message)
	default:
		return apiErr(message)
	}
}
