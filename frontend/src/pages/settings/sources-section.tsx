import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../../lib/api";
import { Card } from "../../components/ui/card";
import { Button } from "../../components/ui/button";
import { Icon } from "../../components/ui/icon";
import { Skeleton } from "../../components/ui/skeleton";
import { ErrorBox } from "../../components/ui/error-box";
import { EmptyState } from "../../components/ui/empty-state";
import { Badge } from "../../components/ui/badge";
import { useTranslation } from "../../i18n";
import type { MsgKey } from "../../i18n/keys";

interface Props {
  workspaceId: string;
}

export function SourcesSection({ workspaceId }: Props) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [url, setUrl] = useState("");
  const [errorKey, setErrorKey] = useState<MsgKey | null>(null);

  const list = useQuery({
    queryKey: ["sources", workspaceId],
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/sources",
        { params: { path: { workspaceId } } },
      );
      if (error) throw error;
      return data;
    },
  });

  const create = useMutation({
    mutationFn: async (input: string) => {
      const { data, error, response } = await api.POST(
        "/api/v1/ws/{workspaceId}/sources",
        {
          params: { path: { workspaceId } },
          body: { provider: "notion", url: input },
        },
      );
      if (error) {
        throw { status: response?.status ?? 0, body: error };
      }
      return data;
    },
    onSuccess: () => {
      setUrl("");
      setErrorKey(null);
      qc.invalidateQueries({ queryKey: ["sources", workspaceId] });
      qc.invalidateQueries({ queryKey: ["tools", workspaceId] });
    },
    onError: (e: { status?: number }) => {
      // The server returns 400/409 with a JSON error body. Map status → key.
      switch (e?.status) {
        case 409:
          setErrorKey("sourcesErrorDuplicate");
          break;
        case 400:
          setErrorKey("sourcesErrorInvalidUrl");
          break;
        default:
          setErrorKey("sourcesErrorGeneric");
      }
    },
  });

  const remove = useMutation({
    mutationFn: async (id: string) => {
      const { error } = await api.DELETE(
        "/api/v1/ws/{workspaceId}/sources/{sourceId}",
        { params: { path: { workspaceId, sourceId: id } } },
      );
      if (error) throw error;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sources", workspaceId] });
      qc.invalidateQueries({ queryKey: ["tools", workspaceId] });
    },
  });

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-[15px] font-semibold mb-1">{t("sourcesTitle")}</h2>
        <p className="text-[13px] text-ink-3">{t("sourcesSubtitle")}</p>
      </div>

      <Card className="p-4 space-y-3">
        <label className="block text-[12px] font-medium text-ink-3">
          {t("sourcesProviderLabel")}: {t("sourcesProviderNotion")}
        </label>
        <div className="flex gap-2">
          <input
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={t("sourcesUrlPlaceholder")}
            className="flex-1 border border-line rounded-2 px-3 py-2 text-[13px] focus:outline-none focus:ring-2 focus:ring-brand"
          />
          <Button
            onClick={() => url && create.mutate(url)}
            disabled={!url || create.isPending}
          >
            {create.isPending ? t("sourcesAdding") : t("sourcesAddButton")}
          </Button>
        </div>
        {errorKey && (
          <p className="text-[12.5px] text-danger">{t(errorKey)}</p>
        )}
      </Card>

      {list.isLoading && (
        <Card className="p-4 space-y-2">
          <Skeleton width="60%" />
          <Skeleton width="40%" />
        </Card>
      )}

      {list.error && (
        <ErrorBox
          title={t("sourcesErrorGeneric")}
          onRetry={() => list.refetch()}
        />
      )}

      {list.data && list.data.sources.length === 0 && (
        <EmptyState title={t("sourcesEmpty")} />
      )}

      {list.data && list.data.sources.length > 0 && (
        <Card className="p-0 overflow-hidden">
          <table className="w-full border-separate border-spacing-0 text-[13px]">
            <thead>
              <tr>
                <th className="text-left px-3 py-2 border-b border-line">
                  {t("sourcesThTitle")}
                </th>
                <th className="text-left px-3 py-2 border-b border-line w-[110px]">
                  {t("sourcesThType")}
                </th>
                <th className="text-left px-3 py-2 border-b border-line w-[170px]">
                  {t("sourcesThAdded")}
                </th>
                <th className="text-right px-3 py-2 border-b border-line w-[110px]">
                  {t("sourcesThActions")}
                </th>
              </tr>
            </thead>
            <tbody>
              {list.data.sources.map((s) => (
                <tr key={s.id} className="[&_td]:border-b [&_td]:border-line last:[&_td]:border-b-0">
                  <td className="px-3 py-2">
                    {s.notion?.url ? (
                      <a
                        href={s.notion.url}
                        target="_blank"
                        rel="noreferrer"
                        className="text-brand hover:underline"
                      >
                        {s.notion.title || s.notion.objectId}
                      </a>
                    ) : (
                      <span>{s.notion?.objectId ?? s.id}</span>
                    )}
                  </td>
                  <td className="px-3 py-2">
                    <Badge tone="neutral" dot={false}>
                      {s.notion?.objectType ?? s.provider}
                    </Badge>
                  </td>
                  <td className="px-3 py-2 text-ink-3">
                    {new Date(s.createdAt).toLocaleString()}
                  </td>
                  <td className="px-3 py-2 text-right">
                    <Button
                      size="sm"
                      onClick={() => {
                        if (window.confirm(t("sourcesDeleteConfirm"))) {
                          remove.mutate(s.id);
                        }
                      }}
                    >
                      <Icon name="trash" size={11} />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}
