package types

type WorkspaceID string
type TicketID string
type CommentID string

func (id WorkspaceID) String() string { return string(id) }
func (id TicketID) String() string    { return string(id) }
func (id CommentID) String() string   { return string(id) }
