package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
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
			switch ev := innerEvent.Data.(type) {
			case *slackevents.MessageEvent:
				if slackUC == nil {
					w.WriteHeader(http.StatusOK)
					return
				}

				if ev.BotID != "" || ev.SubType != "" {
					w.WriteHeader(http.StatusOK)
					return
				}

				if ev.ThreadTimeStamp == "" || ev.ThreadTimeStamp == ev.TimeStamp {
					async.Dispatch(r.Context(), func(ctx context.Context) error {
						return slackUC.HandleNewMessage(ctx, ev.Channel, ev.User, ev.Text, ev.TimeStamp)
					})
				} else {
					async.Dispatch(r.Context(), func(ctx context.Context) error {
						return slackUC.HandleThreadReply(ctx, ev.Channel, ev.ThreadTimeStamp, ev.User, ev.Text, ev.TimeStamp)
					})
				}
			}
			w.WriteHeader(http.StatusOK)

		default:
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
