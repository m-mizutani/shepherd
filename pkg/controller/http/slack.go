package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	slackgo "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

func slackEventHandler(slackUC *usecase.SlackUseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "failed to read request body"), http.StatusBadRequest)
			return
		}

		eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
		if err != nil {
			errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "failed to parse slack event"), http.StatusBadRequest)
			return
		}

		logger := logging.From(r.Context())

		logger.Debug("slack event received",
			slog.String("type", eventsAPIEvent.Type),
		)

		switch eventsAPIEvent.Type {
		case slackevents.URLVerification:
			var challenge slackevents.ChallengeResponse
			if err := json.Unmarshal(body, &challenge); err != nil {
				errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "failed to unmarshal challenge"), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(challenge.Challenge)); err != nil {
				errutil.Handle(r.Context(), goerr.Wrap(err, "failed to write challenge response"))
			}

		case slackevents.CallbackEvent:
			innerEvent := eventsAPIEvent.InnerEvent
			logger.Debug("slack callback event",
				slog.String("inner_type", innerEvent.Type),
			)

			switch ev := innerEvent.Data.(type) {
			case *slackevents.MessageEvent:
				if slackUC == nil {
					logger.Debug("slack event ignored: slackUC is nil")
					w.WriteHeader(http.StatusOK)
					return
				}

				logger.Debug("slack message event",
					slog.String("channel", ev.Channel),
					slog.String("user", ev.User),
					slog.String("ts", ev.TimeStamp),
					slog.String("thread_ts", ev.ThreadTimeStamp),
					slog.String("bot_id", ev.BotID),
					slog.String("subtype", ev.SubType),
				)

				switch ev.SubType {
				case "message_changed":
					if ev.Message != nil {
						async.Dispatch(r.Context(), func(ctx context.Context) error {
							return slackUC.HandleMessageChanged(ctx, ev.Channel, ev.Message.Timestamp, ev.Message.Text)
						})
					}
				case "":
					isBot := ev.BotID != ""
					if ev.ThreadTimeStamp == "" || ev.ThreadTimeStamp == ev.TimeStamp {
						if isBot {
							logger.Debug("slack event skipped: bot message on parent thread")
							w.WriteHeader(http.StatusOK)
							return
						}
						async.Dispatch(r.Context(), func(ctx context.Context) error {
							return slackUC.HandleNewMessage(ctx, ev.Channel, ev.User, ev.Text, ev.TimeStamp)
						})
					} else {
						async.Dispatch(r.Context(), func(ctx context.Context) error {
							return slackUC.HandleThreadReply(ctx, ev.Channel, ev.ThreadTimeStamp, ev.User, ev.Text, ev.TimeStamp, isBot)
						})
					}
				default:
					logger.Debug("slack message subtype skipped",
						slog.String("subtype", ev.SubType),
					)
				}

			default:
				logger.Debug("slack callback event ignored: unhandled inner type",
					slog.String("inner_type", innerEvent.Type),
				)
			}
			w.WriteHeader(http.StatusOK)

		default:
			logger.Debug("slack event ignored: unhandled type",
				slog.String("type", eventsAPIEvent.Type),
			)
			w.WriteHeader(http.StatusOK)
		}
	}
}

func slackSignatureMiddleware(signingSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "failed to read body"), http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			sv, err := slackgo.NewSecretsVerifier(r.Header, signingSecret)
			if err != nil {
				errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "failed to create secrets verifier"), http.StatusUnauthorized)
				return
			}

			if _, err := sv.Write(body); err != nil {
				errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "failed to write body to verifier"), http.StatusUnauthorized)
				return
			}

			if err := sv.Ensure(); err != nil {
				errutil.HandleHTTP(r.Context(), w, goerr.Wrap(err, "slack signature verification failed"), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
