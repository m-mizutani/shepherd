import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { Card } from "../../components/ui/card";
import { Skeleton } from "../../components/ui/skeleton";
import { ErrorBox } from "../../components/ui/error-box";
import { Button } from "../../components/ui/button";
import { Icon, type IconName } from "../../components/ui/icon";
import { useTranslation } from "../../i18n";
import type { MsgKey } from "../../i18n/keys";
import type { components } from "../../generated/api";
import { cn } from "../../lib/utils";
import { PromptsHistoryOverlay } from "./prompts-history-overlay";

type PromptSlot = components["schemas"]["PromptSlot"];

interface Props {
  workspaceId: string;
}

interface SlotMeta {
  id: string;
  icon: IconName;
  labelKey: MsgKey;
  descriptionKey: MsgKey;
}

const SLOT_META: SlotMeta[] = [
  {
    id: "triage",
    icon: "flag",
    labelKey: "promptsSlotTriageLabel",
    descriptionKey: "promptsSlotTriageDescription",
  },
];

export function PromptsSection({ workspaceId }: Props) {
  const { t } = useTranslation();
  const [activeId, setActiveId] = useState<string>(SLOT_META[0].id);

  const slotsQuery = useQuery({
    queryKey: ["prompts", workspaceId],
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/prompts",
        { params: { path: { workspaceId } } },
      );
      if (error) throw error;
      return data;
    },
  });

  return (
    <section className="mb-7">
      <div className="flex items-end justify-between mb-3.5">
        <div>
          <h2 className="m-0 text-[18px] font-semibold tracking-[-0.012em] text-ink-1">
            {t("promptsTitle")}
          </h2>
          <p className="text-[12px] text-ink-3 mt-1 max-w-[640px]">
            {t("promptsSubtitle")}
          </p>
        </div>
        <div className="flex gap-2">
          <Button size="sm" variant="ghost" disabled>
            <Icon name="link" size={12} /> {t("promptsBtnTestOnTicket")}
          </Button>
        </div>
      </div>

      {slotsQuery.isLoading && (
        <Card className="p-4 mb-4">
          <Skeleton width="40%" />
        </Card>
      )}
      {slotsQuery.error && (
        <ErrorBox
          title={t("promptsLoadFailed")}
          onRetry={() => slotsQuery.refetch()}
        />
      )}

      {slotsQuery.data && (
        <>
          <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-2.5 mb-5">
            {slotsQuery.data.prompts.map((slot) => {
              const meta = SLOT_META.find((m) => m.id === slot.id);
              if (!meta) return null;
              return (
                <PromptCard
                  key={slot.id}
                  slot={slot}
                  meta={meta}
                  active={activeId === slot.id}
                  onClick={() => setActiveId(slot.id)}
                />
              );
            })}
          </div>

          {activeId === "triage" && (
            <TriagePromptEditor workspaceId={workspaceId} />
          )}
        </>
      )}
    </section>
  );
}

function PromptCard({
  slot,
  meta,
  active,
  onClick,
}: {
  slot: PromptSlot;
  meta: SlotMeta;
  active: boolean;
  onClick: () => void;
}) {
  const { t } = useTranslation();
  const empty = !slot.configured;
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "text-left p-3 bg-bg-elev rounded-2 cursor-pointer flex flex-col gap-1.5",
        active
          ? "border-[1.5px] border-brand shadow-[0_0_0_3px_var(--brand-soft)]"
          : "border border-line shadow-1",
        empty && "opacity-85",
      )}
    >
      <div className="flex items-center gap-1.5">
        <span
          className={cn(
            "w-[22px] h-[22px] rounded-1 inline-flex items-center justify-center",
            active
              ? "bg-brand-soft text-brand"
              : "bg-bg-sunken text-ink-3",
          )}
        >
          <Icon name={meta.icon} size={12} />
        </span>
        <span className="text-[13px] font-semibold text-ink-1">
          {t(meta.labelKey)}
        </span>
      </div>
      <div className="text-[11.5px] leading-tight text-ink-3">
        {t(meta.descriptionKey)}
      </div>
      <div className="flex items-center gap-2 mt-0.5 min-h-[18px] text-[11px] text-ink-3">
        {empty ? (
          <span className="italic text-ink-4">
            {t("promptsSlotNotConfigured")}
          </span>
        ) : (
          <>
            <span>
              {t("promptsSlotCharsCount", { count: String(slot.length) })}
            </span>
            <span className="flex-1" />
            {slot.updatedBy && (
              <span className="font-medium text-ink-2">
                @{slot.updatedBy.name}
              </span>
            )}
            {slot.updatedAt && (
              <span>{formatRelative(new Date(slot.updatedAt))}</span>
            )}
          </>
        )}
      </div>
    </button>
  );
}

function TriagePromptEditor({ workspaceId }: Props) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const promptKey = ["prompts", workspaceId, "triage"] as const;

  const detail = useQuery({
    queryKey: promptKey,
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/prompts/{promptId}",
        { params: { path: { workspaceId, promptId: "triage" } } },
      );
      if (error) throw error;
      return data;
    },
  });

  const [draft, setDraft] = useState<string>("");
  const [errorBanner, setErrorBanner] = useState<{
    kind: "conflict" | "generic";
    currentVersion?: number;
  } | null>(null);
  const [historyOpen, setHistoryOpen] = useState(false);

  // When the server data arrives (or refreshes after a save / restore /
  // conflict-driven reload), reset the local draft to the server's content.
  useEffect(() => {
    if (detail.data) {
      setDraft(detail.data.content);
      setErrorBanner(null);
    }
  }, [detail.data]);

  const save = useMutation({
    mutationFn: async (content: string) => {
      const nextVersion = (detail.data?.version ?? 0) + 1;
      const { data, error, response } = await api.PUT(
        "/api/v1/ws/{workspaceId}/prompts/{promptId}",
        {
          params: { path: { workspaceId, promptId: "triage" } },
          body: { content, version: nextVersion },
        },
      );
      if (error) {
        // openapi-fetch surfaces the body as `error` for non-2xx; status is
        // on the raw response so we can route 422 vs 409 correctly.
        throw { error, status: response.status };
      }
      return data;
    },
    onSuccess: async () => {
      setErrorBanner(null);
      await qc.invalidateQueries({ queryKey: promptKey });
      await qc.invalidateQueries({ queryKey: ["prompts", workspaceId] });
      await qc.invalidateQueries({ queryKey: [...promptKey, "history"] });
    },
    onError: (err: unknown) => {
      const e = err as { status?: number; error?: unknown };
      if (e.status === 409) {
        const body = e.error as { currentVersion?: number } | undefined;
        setErrorBanner({
          kind: "conflict",
          currentVersion: body?.currentVersion ?? -1,
        });
      } else {
        setErrorBanner({ kind: "generic" });
      }
    },
  });

  if (detail.isLoading) {
    return (
      <Card className="p-4">
        <Skeleton width="100%" height={32} />
        <div className="mt-3">
          <Skeleton width="100%" height={120} />
        </div>
      </Card>
    );
  }
  if (detail.error || !detail.data) {
    return (
      <ErrorBox title={t("promptsLoadFailed")} onRetry={() => detail.refetch()} />
    );
  }

  const dirty = draft !== detail.data.content;
  const lineCount = draft.split("\n").length;
  const charCount = draft.length;

  const reloadFromServer = () => {
    setDraft(detail.data!.content);
    setErrorBanner(null);
    detail.refetch();
  };

  return (
    <>
      <Card className="p-0 overflow-hidden">
        <div className="flex items-center gap-2.5 px-3.5 py-2.5 bg-bg-elev border-b border-line">
          <span className="w-6 h-6 rounded-2 bg-brand-soft text-brand inline-flex items-center justify-center">
            <Icon name="flag" size={13} />
          </span>
          <div className="flex flex-col">
            <span className="text-[13.5px] font-semibold">
              {t("promptsEditorTriageHeading")}
            </span>
            <span className="text-[11px] text-ink-3">
              {t("promptsEditorSubtitleTriage")}
            </span>
          </div>
          <div className="flex-1" />
          <Button
            size="sm"
            variant="ghost"
            onClick={() => setHistoryOpen(true)}
          >
            <Icon name="sort" size={12} /> {t("promptsEditorBtnHistory")}
          </Button>
        </div>

        <div className="px-3 py-1.5 bg-bg-sunken border-b border-line text-[11px] text-ink-3 flex items-center justify-end">
          <span>
            {t("promptsEditorMetaCharsLines", {
              chars: String(charCount),
              lines: String(lineCount),
            })}
          </span>
        </div>

        <CodeArea
          value={draft}
          onChange={setDraft}
          placeholder={t("promptsEditorPlaceholder")}
        />

        <div className="px-3.5 py-2.5 bg-bg-sunken border-t border-line text-[11.5px] text-ink-3">
          {t("promptsEditorAdditionalGuidanceHint")}
        </div>

        <div className="px-3.5 py-3 bg-bg-elev border-t border-line flex items-center gap-2.5">
          <span className="inline-flex items-center gap-1.5 text-[12.5px] font-medium">
            <span
              className={cn(
                "w-2 h-2 rounded-full",
                dirty ? "bg-brand" : "bg-success",
              )}
            />
            {dirty ? t("promptsEditorUnsaved") : t("promptsEditorClean")}
          </span>
          <div className="flex-1" />
          <Button
            size="sm"
            variant="ghost"
            disabled={!dirty || save.isPending}
            onClick={() => setDraft(detail.data!.content)}
          >
            {t("promptsEditorBtnDiscard")}
          </Button>
          <Button
            size="sm"
            variant="primary"
            disabled={!dirty || save.isPending}
            onClick={() => save.mutate(draft)}
          >
            <Icon name="check" size={12} />
            {save.isPending
              ? t("promptsEditorBtnSaving")
              : t("promptsEditorBtnSave")}
          </Button>
        </div>

        {errorBanner && (
          <ErrorBanner
            banner={errorBanner}
            onReload={reloadFromServer}
            onDismiss={() => setErrorBanner(null)}
          />
        )}
      </Card>

      {historyOpen && (
        <PromptsHistoryOverlay
          workspaceId={workspaceId}
          promptId="triage"
          onClose={() => setHistoryOpen(false)}
          onRestored={async () => {
            await qc.invalidateQueries({ queryKey: promptKey });
            await qc.invalidateQueries({ queryKey: ["prompts", workspaceId] });
          }}
        />
      )}
    </>
  );
}

interface ErrorBannerData {
  kind: "conflict" | "generic";
  currentVersion?: number;
}

function ErrorBanner({
  banner,
  onReload,
  onDismiss,
}: {
  banner: ErrorBannerData;
  onReload: () => void;
  onDismiss: () => void;
}) {
  const { t } = useTranslation();
  let message = t("promptsEditorErrorGeneric");
  if (banner.kind === "conflict") {
    message = t("promptsEditorErrorVersionConflict", {
      version: String(banner.currentVersion ?? 0),
    });
  }
  return (
    <div className="px-3.5 py-2.5 bg-danger-soft border-t border-line flex items-center gap-2 text-[12.5px] text-danger">
      <Icon name="alert" size={13} />
      <span className="flex-1">{message}</span>
      {banner.kind === "conflict" && (
        <Button size="sm" variant="default" onClick={onReload}>
          {t("promptsEditorReloadAndDiscard")}
        </Button>
      )}
      <Button size="sm" variant="ghost" onClick={onDismiss}>
        <Icon name="x" size={12} />
      </Button>
    </div>
  );
}

function CodeArea({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  const lines = useMemo(() => Math.max(1, value.split("\n").length), [value]);
  return (
    <div
      className="flex font-mono text-[12.5px] leading-[1.6] bg-[#fdfcf9]"
      style={{ minHeight: 540 }}
    >
      <div
        className="select-none text-right text-ink-5 bg-[#faf8f3] border-r border-line py-3 px-2"
        style={{ flex: "0 0 44px" }}
      >
        {Array.from({ length: lines }, (_, i) => (
          <div key={i} style={{ height: "1.6em" }}>
            {i + 1}
          </div>
        ))}
      </div>
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        spellCheck={false}
        className="flex-1 py-3 px-4 bg-transparent text-ink-2 font-mono text-[12.5px] leading-[1.6] resize-none border-0 outline-none focus:outline-none"
        style={{ minHeight: 540 }}
      />
    </div>
  );
}

function formatRelative(date: Date): string {
  const now = Date.now();
  const diffMs = now - date.getTime();
  const sec = Math.round(diffMs / 1000);
  if (sec < 60) return `${sec}s ago`;
  const min = Math.round(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.round(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.round(hr / 24);
  if (day < 30) return `${day}d ago`;
  return date.toLocaleDateString();
}
