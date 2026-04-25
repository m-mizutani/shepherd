package firestore

import (
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func isNotFound(err error) bool {
	return status.Code(err) == codes.NotFound
}

func toTime(v any) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t
	default:
		return time.Time{}
	}
}
