package slack

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
)

type getUserInfoTool struct {
	slack SlackTooler
}

func newGetUserInfoTool(s SlackTooler) gollem.Tool {
	return &getUserInfoTool{slack: s}
}

func (t *getUserInfoTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "slack_get_user_info",
		Description: "Resolve a Slack user ID (e.g. 'U0123456') to a human-readable display name and email. Use this to translate user mentions found in messages or tickets into people.",
		Parameters: map[string]*gollem.Parameter{
			"user_id": {
				Type:        gollem.TypeString,
				Description: "Slack user ID.",
				Required:    true,
				MinLength:   ptrInt(1),
			},
		},
	}
}

func (t *getUserInfoTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	userID, err := stringArg(args, "user_id", true)
	if err != nil {
		return nil, err
	}
	info, err := t.slack.GetUserInfo(ctx, userID)
	if err != nil {
		return nil, goerr.Wrap(err, "slack_get_user_info failed",
			goerr.V("user_id", userID))
	}
	if info == nil {
		return map[string]any{"found": false}, nil
	}
	return map[string]any{
		"found":     true,
		"id":        info.ID,
		"name":      info.Name,
		"email":     info.Email,
		"image_url": info.ImageURL,
	}, nil
}
