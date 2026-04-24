package errutil

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/getsentry/sentry-go"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

func Handle(ctx context.Context, err error) {
	if err == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "[CRITICAL] slog crashed during error handling: original_error=%s, slog_panic=%v\n",
				err.Error(), r)
		}
	}()

	logAttrs := []any{slog.Any("error", err)}
	logger := logging.From(ctx)

	hub := sentry.CurrentHub().Clone()
	hub.ConfigureScope(func(scope *sentry.Scope) {
		values := map[string]any{}
		for k, v := range goerr.Values(err) {
			values[k] = v
		}
		if len(values) > 0 {
			scope.SetContext("goerr_values", values)
		}
	})
	evID := hub.CaptureException(err)
	logAttrs = append(logAttrs, slog.Any("sentry.id", evID))

	logger.Error("Error: "+err.Error(), logAttrs...)
}

func HandleHTTP(ctx context.Context, w http.ResponseWriter, err error, statusCode int) {
	if err == nil {
		return
	}

	Handle(ctx, err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	fmt.Fprintf(w, `{"error":%q}`, err.Error())
}
