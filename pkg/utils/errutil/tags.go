package errutil

import "github.com/m-mizutani/goerr/v2"

var (
	// Client errors (4xx)
	TagNotFound     = goerr.NewTag("not_found")
	TagValidation   = goerr.NewTag("validation")
	TagUnauthorized = goerr.NewTag("unauthorized")
	TagForbidden    = goerr.NewTag("forbidden")
	TagConflict     = goerr.NewTag("conflict")

	// Server errors (5xx)
	TagInternal = goerr.NewTag("internal")
	TagExternal = goerr.NewTag("external")
	TagTimeout  = goerr.NewTag("timeout")
	TagDatabase = goerr.NewTag("database")

	// Business logic errors
	TagInvalidState      = goerr.NewTag("invalid_state")
	TagDuplicateResource = goerr.NewTag("duplicate_resource")

	// External service errors
	TagSlackError = goerr.NewTag("slack_error")
)
