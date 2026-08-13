package contentsync

import "errors"

var (
	ErrNotConfigured   = errors.New("content sync is not configured")
	ErrUnauthorized    = errors.New("invalid content sync signature")
	ErrForbiddenRepo   = errors.New("repository is not allowed")
	ErrStaleTimestamp  = errors.New("timestamp is outside the allowed window")
	ErrDuplicateRun    = errors.New("content sync run was already applied")
	ErrInvalidSnapshot = errors.New("invalid content snapshot")
	ErrMissingImage    = errors.New("referenced image is not in storage")
)
