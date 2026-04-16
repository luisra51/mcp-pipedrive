package pipedrive

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrMissingClient     = errors.New("pipedrive client not available in context")
	ErrMissingAuth       = errors.New("no credential configured: set PIPEDRIVE_API_TOKEN or PIPEDRIVE_OAUTH_ACCESS_TOKEN")
	ErrWriteDisabled     = errors.New("write operations are disabled (set PIPEDRIVE_ALLOW_WRITE=true)")
	ErrDeleteDisabled    = errors.New("delete operations are disabled (set PIPEDRIVE_ALLOW_DELETE=true)")
	ErrAdminToolDisabled = errors.New("admin tools are disabled (set PIPEDRIVE_ENABLE_ADMIN_TOOLS=true)")
)

// APIError carries the HTTP status and body of a non-2xx Pipedrive response.
type APIError struct {
	StatusCode int
	Status     string
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("pipedrive api error: %s", e.Status)
}

// IsRateLimited reports whether the upstream returned 429.
func (e *APIError) IsRateLimited() bool { return e.StatusCode == http.StatusTooManyRequests }

// IsServerError reports whether the upstream returned a 5xx.
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= http.StatusInternalServerError && e.StatusCode < 600
}
