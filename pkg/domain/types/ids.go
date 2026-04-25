package types

type WorkspaceID string
type TicketID string
type CommentID string
type FieldID string
type SlackChannelID string
type SlackThreadTS string
type SlackUserID string
type StatusID string

func (id WorkspaceID) String() string   { return string(id) }
func (id TicketID) String() string      { return string(id) }
func (id CommentID) String() string     { return string(id) }
func (id FieldID) String() string       { return string(id) }
func (id SlackChannelID) String() string { return string(id) }
func (id SlackThreadTS) String() string  { return string(id) }
func (id SlackUserID) String() string    { return string(id) }
func (id StatusID) String() string       { return string(id) }
