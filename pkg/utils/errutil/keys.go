package errutil

import (
	"time"

	"github.com/m-mizutani/goerr/v2"
)

var (
	// IDs
	TicketIDKey    = goerr.NewTypedKey[string]("ticket_id")
	CommentIDKey   = goerr.NewTypedKey[string]("comment_id")
	WorkspaceIDKey = goerr.NewTypedKey[string]("workspace_id")
	UserIDKey      = goerr.NewTypedKey[string]("user_id")
	RequestIDKey   = goerr.NewTypedKey[string]("request_id")
	TokenIDKey     = goerr.NewTypedKey[string]("token_id")

	// Field names
	FieldKey        = goerr.NewTypedKey[string]("field")
	ParameterKey    = goerr.NewTypedKey[string]("parameter")
	FunctionNameKey = goerr.NewTypedKey[string]("function_name")

	// Values
	StatusKey     = goerr.NewTypedKey[string]("status")
	OperationKey  = goerr.NewTypedKey[string]("operation")
	RepositoryKey = goerr.NewTypedKey[string]("repository")
	CollectionKey = goerr.NewTypedKey[string]("collection")
	DurationKey   = goerr.NewTypedKey[time.Duration]("duration")

	// HTTP
	HTTPStatusKey = goerr.NewTypedKey[int]("http_status")
	URLKey        = goerr.NewTypedKey[string]("url")

	// Slack
	ChannelIDKey = goerr.NewTypedKey[string]("channel_id")
	MessageTSKey = goerr.NewTypedKey[string]("message_ts")
	SlackUserKey = goerr.NewTypedKey[string]("slack_user")

	// File and path
	FilePathKey = goerr.NewTypedKey[string]("file_path")
)
