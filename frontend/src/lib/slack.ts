/** Build a "deep link" URL to open a Slack thread message. */
export function slackThreadUrl(
  channelId?: string | null,
  threadTs?: string | null,
): string | null {
  if (!channelId || !threadTs) return null;
  const params = new URLSearchParams({
    channel: channelId,
    message_ts: threadTs,
  });
  return `https://slack.com/app_redirect?${params.toString()}`;
}
