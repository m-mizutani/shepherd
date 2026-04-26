import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../../../lib/api";
import { Card } from "../../../components/ui/card";
import { Button } from "../../../components/ui/button";
import { Dialog } from "../../../components/ui/dialog";
import { Icon } from "../../../components/ui/icon";
import { Skeleton } from "../../../components/ui/skeleton";
import { ErrorBox } from "../../../components/ui/error-box";
import { EmptyState } from "../../../components/ui/empty-state";
import { Badge } from "../../../components/ui/badge";
import { useTranslation } from "../../../i18n";
import type { MsgKey } from "../../../i18n/keys";

interface Props {
  workspaceId: string;
}

export function NotionSourcesPanel({ workspaceId }: Props) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [dialogOpen, setDialogOpen] = useState(false);

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

  const notionSources =
    list.data?.sources.filter((s) => s.provider === "notion") ?? [];

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Icon name="book" size={14} className="text-ink-3" />
          <h3 className="text-[13px] font-semibold">{t("sourcesProviderNotion")}</h3>
        </div>
        <Button size="sm" onClick={() => setDialogOpen(true)}>
          <Icon name="plus" size={11} /> {t("sourcesAddButton")}
        </Button>
      </div>

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

      {!list.isLoading && !list.error && notionSources.length === 0 && (
        <EmptyState title={t("sourcesEmptyNotion")} />
      )}

      {notionSources.length > 0 && (
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
                <th className="text-left px-3 py-2 border-b border-line">
                  {t("sourcesThDescription")}
                </th>
                <th className="text-left px-3 py-2 border-b border-line w-[170px]">
                  {t("sourcesThAdded")}
                </th>
                <th className="text-right px-3 py-2 border-b border-line w-[70px]">
                  {t("sourcesThActions")}
                </th>
              </tr>
            </thead>
            <tbody>
              {notionSources.map((s) => (
                <tr key={s.id} className="[&_td]:border-b [&_td]:border-line last:[&_td]:border-b-0 align-top">
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
                  <td className="px-3 py-2 text-ink-2">
                    {s.description ? (
                      <span className="whitespace-pre-wrap">{s.description}</span>
                    ) : (
                      <span className="text-ink-4">—</span>
                    )}
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

      <AddNotionSourceDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        workspaceId={workspaceId}
      />
    </section>
  );
}

interface DialogProps {
  open: boolean;
  onClose: () => void;
  workspaceId: string;
}

function AddNotionSourceDialog({ open, onClose, workspaceId }: DialogProps) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const [url, setUrl] = useState("");
  const [description, setDescription] = useState("");
  const [errorKey, setErrorKey] = useState<MsgKey | null>(null);

  const reset = () => {
    setUrl("");
    setDescription("");
    setErrorKey(null);
  };

  const create = useMutation({
    mutationFn: async () => {
      const { data, error, response } = await api.POST(
        "/api/v1/ws/{workspaceId}/sources",
        {
          params: { path: { workspaceId } },
          body: {
            provider: "notion",
            url,
            ...(description ? { description } : {}),
          },
        },
      );
      if (error) {
        throw { status: response?.status ?? 0, body: error };
      }
      return data;
    },
    onSuccess: () => {
      reset();
      qc.invalidateQueries({ queryKey: ["sources", workspaceId] });
      qc.invalidateQueries({ queryKey: ["tools", workspaceId] });
      onClose();
    },
    onError: (e: { status?: number }) => {
      switch (e?.status) {
        case 400:
          setErrorKey("sourcesErrorInvalidUrl");
          break;
        case 403:
          setErrorKey("sourcesErrorForbidden");
          break;
        case 404:
          setErrorKey("sourcesErrorNotFound");
          break;
        case 409:
          setErrorKey("sourcesErrorDuplicate");
          break;
        default:
          setErrorKey("sourcesErrorGeneric");
      }
    },
  });

  return (
    <Dialog
      open={open}
      onClose={() => {
        reset();
        onClose();
      }}
      title={t("sourcesAddDialogTitle")}
      width={520}
      footer={
        <>
          <Button
            onClick={() => {
              reset();
              onClose();
            }}
          >
            {t("btnCancel")}
          </Button>
          <Button
            onClick={() => url && create.mutate()}
            disabled={!url || create.isPending}
          >
            {create.isPending ? t("sourcesAdding") : t("sourcesAddButton")}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <div className="space-y-1">
          <label className="block text-[11.5px] font-medium text-ink-3">
            {t("sourcesNotionUrlLabel")}
          </label>
          <input
            type="url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder={t("sourcesUrlPlaceholder")}
            autoFocus
            className="w-full border border-line rounded-2 px-3 py-2 text-[13px] focus:outline-none focus:ring-2 focus:ring-brand"
          />
        </div>
        <div className="space-y-1">
          <label className="block text-[11.5px] font-medium text-ink-3">
            {t("sourcesDescriptionLabel")}
          </label>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder={t("sourcesDescriptionPlaceholder")}
            rows={3}
            className="w-full border border-line rounded-2 px-3 py-2 text-[13px] focus:outline-none focus:ring-2 focus:ring-brand"
          />
          <p className="text-[11.5px] text-ink-3">
            {t("sourcesDescriptionHint")}
          </p>
        </div>
        {errorKey && (
          <p className="text-[12.5px] text-danger">{t(errorKey)}</p>
        )}
      </div>
    </Dialog>
  );
}
