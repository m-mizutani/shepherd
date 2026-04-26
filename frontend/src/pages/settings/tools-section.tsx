import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { Card } from "../../components/ui/card";
import { Skeleton } from "../../components/ui/skeleton";
import { ErrorBox } from "../../components/ui/error-box";
import { useTranslation } from "../../i18n";
import type { MsgKey } from "../../i18n/keys";
import type { components } from "../../generated/api";

interface Props {
  workspaceId: string;
}

type ToolState = components["schemas"]["ToolState"];
type ToolsResponse = { tools: ToolState[] };

const PROVIDER_LABEL: Record<string, MsgKey> = {
  meta: "toolsProviderMeta",
  ticket: "toolsProviderTicket",
  slack: "toolsProviderSlack",
  notion: "toolsProviderNotion",
};

const REASON_LABEL: Record<string, MsgKey> = {
  provider_unavailable: "toolsReasonProviderUnavailable",
  workspace_disabled: "toolsReasonWorkspaceDisabled",
  gate_blocked: "toolsReasonGateBlocked",
};

export function ToolsSection({ workspaceId }: Props) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const queryKey = ["tools", workspaceId] as const;

  const list = useQuery({
    queryKey,
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/tools",
        { params: { path: { workspaceId } } },
      );
      if (error) throw error;
      return data;
    },
  });

  const set = useMutation({
    mutationFn: async (input: { providerId: string; enabled: boolean }) => {
      const { error } = await api.PUT(
        "/api/v1/ws/{workspaceId}/tools/{providerId}",
        {
          params: { path: { workspaceId, providerId: input.providerId } },
          body: { enabled: input.enabled },
        },
      );
      if (error) throw error;
    },
    // Optimistically flip the checkbox the moment the user clicks so the UI
    // never feels frozen waiting for the round-trip + invalidate. We still
    // refetch onSettled to pick up server-side side effects (e.g. notion
    // gate flipping a row to gate_blocked when its source list changes).
    onMutate: async (input) => {
      await qc.cancelQueries({ queryKey });
      const prev = qc.getQueryData<ToolsResponse>(queryKey);
      qc.setQueryData<ToolsResponse>(queryKey, (cur) => {
        if (!cur) return cur;
        return {
          tools: cur.tools.map((t) =>
            t.providerId === input.providerId
              ? { ...t, enabled: input.enabled }
              : t,
          ),
        };
      });
      return { prev };
    },
    onError: (_err, _input, ctx) => {
      if (ctx?.prev) qc.setQueryData(queryKey, ctx.prev);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey });
    },
  });

  // Track which row is in-flight so we can show a per-row "Saving…" label.
  // mutation.variables holds the most recent input while isPending is true.
  const pendingProviderId =
    set.isPending ? set.variables?.providerId : undefined;

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-[15px] font-semibold mb-1">{t("toolsTitle")}</h2>
        <p className="text-[13px] text-ink-3">{t("toolsSubtitle")}</p>
      </div>

      {list.isLoading && (
        <Card className="p-4 space-y-2">
          <Skeleton width="60%" />
          <Skeleton width="40%" />
        </Card>
      )}

      {list.error && (
        <ErrorBox title="" onRetry={() => list.refetch()} />
      )}

      {list.data && (
        <Card className="p-0 overflow-hidden">
          <table className="w-full border-separate border-spacing-0 text-[13px]">
            <tbody>
              {list.data.tools.map((tool) => {
                const labelKey = PROVIDER_LABEL[tool.providerId];
                const reasonKey = tool.reason ? REASON_LABEL[tool.reason] : null;
                const togglable = tool.available;
                const saving = pendingProviderId === tool.providerId;
                return (
                  <tr
                    key={tool.providerId}
                    className="[&_td]:border-b [&_td]:border-line last:[&_td]:border-b-0"
                  >
                    <td className="px-3 py-3 align-top">
                      <div className="font-medium">
                        {labelKey ? t(labelKey) : tool.providerId}
                      </div>
                      {reasonKey && (
                        <div className="text-[12px] text-ink-3 mt-0.5">
                          {t(reasonKey)}
                        </div>
                      )}
                    </td>
                    <td className="px-3 py-3 text-right align-top w-[160px]">
                      <label className="inline-flex items-center gap-2 cursor-pointer select-none">
                        <input
                          type="checkbox"
                          checked={tool.enabled}
                          disabled={!togglable || saving}
                          onChange={(e) =>
                            set.mutate({
                              providerId: tool.providerId,
                              enabled: e.target.checked,
                            })
                          }
                        />
                        <span
                          className={
                            saving
                              ? "text-[12px] text-ink-3 italic inline-flex items-center gap-1.5"
                              : "text-[12px] text-ink-3"
                          }
                        >
                          {saving && <Spinner />}
                          {saving
                            ? t("toolsToggleSaving")
                            : t("toolsToggleEnabled")}
                        </span>
                      </label>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}

function Spinner() {
  return (
    <span
      aria-hidden
      className="inline-block w-3 h-3 border-2 border-ink-4 border-t-transparent rounded-full animate-spin"
    />
  );
}
