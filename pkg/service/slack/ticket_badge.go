package slack

import (
	"context"

	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
	slackgo "github.com/slack-go/slack"
)

// TicketRef carries the minimum metadata every ticket-scoped Slack message
// needs to render the reference line at the top. Centralising this shape
// keeps the per-state rendering logic (Active / Inactive / Dismissed) in
// one place — every Build* function delegates to it instead of constructing
// ad-hoc title strings.
type TicketRef struct {
	ID     types.TicketID
	SeqNum int64
	Title  string
	URL    string
}

// TicketRefState selects the visual treatment of the ticket reference line.
// The state encodes the message's lifecycle role rather than its content,
// so a thread reader can tell at a glance which single message represents
// the ticket's live state.
type TicketRefState int

const (
	// TicketRefStateActive marks the message as the ticket's current live
	// state — the one demanding reader attention or representing the
	// terminal outcome. Renders prominently (bold section + 🎫 marker).
	// At most one Active reference should appear in any given thread at a
	// time; everything else is Inactive or Dismissed.
	TicketRefStateActive TicketRefState = iota
	// TicketRefStateInactive marks historical or transitional messages
	// (progress, hand-off, submitted, retry-queued, ask-answered, …).
	// Renders as a quiet context block so old messages stop competing for
	// the reader's attention.
	TicketRefStateInactive
	// TicketRefStateDismissed marks the review proposal a reporter sent
	// back via Re-investigate. Renders struck-through so the rejected
	// proposal is visibly invalidated.
	TicketRefStateDismissed
)

// ticketRefBlock returns the single block representing the ticket reference,
// styled per state. Returns nil when ref carries no SeqNum (e.g. legacy
// tests with a bare TicketRef) so callers stay readable. The block kind is
// chosen for visual weight: SectionBlock for Active (regular body size,
// bold), ContextBlock for Inactive / Dismissed (small dim).
func ticketRefBlock(ctx context.Context, ref TicketRef, state TicketRefState) slackgo.Block {
	if ref.SeqNum == 0 {
		return nil
	}
	loc := i18n.From(ctx)
	var key i18n.MsgKey
	switch state {
	case TicketRefStateActive:
		key = i18n.MsgTicketRefActive
	case TicketRefStateDismissed:
		key = i18n.MsgTicketRefDismissed
	default:
		key = i18n.MsgTicketRefInactive
	}
	text := loc.T(key,
		"url", ref.URL,
		"id", ref.SeqNum,
		"title", ref.Title,
	)
	if state == TicketRefStateActive {
		return slackgo.NewSectionBlock(
			slackgo.NewTextBlockObject(slackgo.MarkdownType, "*"+text+"*", false, false),
			nil, nil,
		)
	}
	return slackgo.NewContextBlock("ticket_ref",
		slackgo.NewTextBlockObject(slackgo.MarkdownType, text, false, false),
	)
}

// prependTicketRef returns blocks with the ticket reference inserted at
// index 0. When ref has no SeqNum (would render nothing), the input is
// returned unchanged so callers stay readable.
func prependTicketRef(ctx context.Context, ref TicketRef, state TicketRefState, blocks []slackgo.Block) []slackgo.Block {
	block := ticketRefBlock(ctx, ref, state)
	if block == nil {
		return blocks
	}
	out := make([]slackgo.Block, 0, len(blocks)+1)
	out = append(out, block)
	out = append(out, blocks...)
	return out
}
