import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { Icon } from "../../components/ui/icon";
import { Button } from "../../components/ui/button";
import { Skeleton } from "../../components/ui/skeleton";
import { ErrorBox } from "../../components/ui/error-box";
import { useTranslation } from "../../i18n";
import { cn } from "../../lib/utils";
import type { components } from "../../generated/api";
import { diffLines, type DiffHunk, type DiffResult } from "../../lib/diff";

type PromptVersion = components["schemas"]["PromptVersion"];

interface Props {
  workspaceId: string;
  promptId: "triage";
  onClose: () => void;
  onRestored: () => Promise<void> | void;
}

export function PromptsHistoryOverlay({
  workspaceId,
  promptId,
  onClose,
  onRestored,
}: Props) {
  const { t } = useTranslation();
  const qc = useQueryClient();

  const historyKey = ["prompts", workspaceId, promptId, "history"] as const;
  const history = useQuery({
    queryKey: historyKey,
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/prompts/{promptId}/history",
        { params: { path: { workspaceId, promptId } } },
      );
      if (error) throw error;
      return data;
    },
  });

  // Picking semantics: "comparing" = (left -> right). Default to (prev -> current).
  const versions = history.data?.versions ?? [];
  const sortedDesc = useMemo(
    () => [...versions].sort((a, b) => b.version - a.version),
    [versions],
  );
  const current = sortedDesc[0];
  const previous = sortedDesc[1];

  const [leftVersion, setLeftVersion] = useState<number | null>(null);
  const [rightVersion, setRightVersion] = useState<number | null>(null);

  // Initialize / re-sync the comparison once data arrives.
  useEffect(() => {
    if (!current) return;
    setRightVersion((v) => v ?? current.version);
    setLeftVersion((v) => v ?? previous?.version ?? null);
  }, [current, previous]);

  const left = sortedDesc.find((v) => v.version === leftVersion) ?? null;
  const right = sortedDesc.find((v) => v.version === rightVersion) ?? null;
  const diff: DiffResult | null = useMemo(() => {
    if (!right) return null;
    return diffLines(left?.content ?? "", right.content);
  }, [left, right]);

  const restore = useMutation({
    mutationFn: async (targetVersion: number) => {
      const nextVersion = (current?.version ?? 0) + 1;
      const { data, error } = await api.POST(
        "/api/v1/ws/{workspaceId}/prompts/{promptId}/restore",
        {
          params: { path: { workspaceId, promptId } },
          body: { targetVersion, version: nextVersion },
        },
      );
      if (error) throw error;
      return data;
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: historyKey });
      await onRestored();
    },
  });

  return (
    <div
      className="fixed inset-0 z-30 flex items-stretch justify-end bg-black/30 backdrop-blur-[2px]"
      onClick={onClose}
    >
      <div
        className="w-[min(1080px,92%)] h-full bg-bg border-l border-line shadow-[-12px_0_40px_rgba(0,0,0,0.18)] flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-5 py-3.5 flex items-center gap-3 bg-bg-elev border-b border-line">
          <span className="w-7 h-7 rounded-2 bg-brand-soft text-brand inline-flex items-center justify-center">
            <Icon name="sort" size={14} />
          </span>
          <div>
            <div className="text-[14px] font-semibold">
              {t("promptsHistoryTitle", {
                label: promptId === "triage" ? "Triage" : promptId,
              })}
            </div>
            <div className="text-[11.5px] text-ink-3">
              {t("promptsHistorySubtitle", {
                count: String(versions.length),
              })}
            </div>
          </div>
          <div className="flex-1" />
          <Button size="sm" variant="ghost" onClick={onClose}>
            <Icon name="x" size={12} />
          </Button>
        </div>

        {history.isLoading && (
          <div className="p-6">
            <Skeleton width="50%" />
          </div>
        )}
        {history.error && (
          <div className="p-6">
            <ErrorBox
              title={t("promptsLoadFailed")}
              onRetry={() => history.refetch()}
            />
          </div>
        )}

        {history.data && versions.length === 0 && (
          <div className="p-8 text-center text-ink-3 text-[13px]">
            {t("promptsHistoryEmpty")}
          </div>
        )}

        {history.data && versions.length > 0 && (
          <div className="flex-1 grid grid-cols-[320px_1fr] min-h-0">
            <div className="border-r border-line bg-bg-elev overflow-auto">
              <div className="px-3.5 py-2.5">
                <div className="text-[11px] font-semibold uppercase tracking-[0.04em] text-ink-3">
                  {t("promptsHistoryVersionsHeader")}
                </div>
              </div>
              {sortedDesc.map((v) => (
                <VersionRow
                  key={v.version}
                  version={v}
                  isLive={v.version === current?.version}
                  selected={v.version === leftVersion}
                  onSelect={() => setLeftVersion(v.version)}
                />
              ))}
            </div>

            <div className="flex flex-col min-h-0">
              <div className="px-4 py-2.5 flex items-center gap-2.5 border-b border-line bg-bg-elev">
                <span className="text-[11.5px] font-medium text-ink-3">
                  {t("promptsHistoryComparing")}
                </span>
                <VersionPill v={left} />
                <Icon name="arrow" size={12} className="text-ink-4" />
                <VersionPill v={right} current />
                <div className="flex-1" />
                {diff && (
                  <span className="text-[11.5px] font-medium">
                    <span className="text-success">+{diff.add}</span>
                    <span className="text-danger ml-1">−{diff.del}</span>
                  </span>
                )}
                {left && (
                  <Button
                    size="sm"
                    variant="default"
                    disabled={restore.isPending}
                    onClick={() => restore.mutate(left.version)}
                  >
                    <Icon name="arrow" size={12} className="rotate-180" />
                    {restore.isPending
                      ? t("promptsHistoryRestoreInProgress")
                      : t("promptsHistoryBtnRestore", {
                          version: String(left.version),
                        })}
                  </Button>
                )}
              </div>

              {right && (
                <div className="px-4 py-3 flex items-center gap-2.5 border-b border-line">
                  <div className="w-7 h-7 rounded-full bg-bg-sunken inline-flex items-center justify-center">
                    <Icon name="user" size={13} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-[13px] font-medium truncate">
                      {right.updatedBy?.name ?? "—"}
                    </div>
                    <div className="text-[11.5px] text-ink-3">
                      v{right.version} ·{" "}
                      {new Date(right.updatedAt).toLocaleString()}
                    </div>
                  </div>
                </div>
              )}

              <div className="flex-1 overflow-auto bg-[#fdfcf9] font-mono text-[12.5px] leading-[1.6] pb-2">
                {diff && diff.hunks.length === 0 && (
                  <div className="p-6 text-ink-4 text-center text-[12.5px]">
                    {t("promptsHistoryHunkUnchanged")}
                  </div>
                )}
                {diff?.hunks.map((hunk, i) => (
                  <DiffHunkView key={i} hunk={hunk} />
                ))}
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function VersionRow({
  version,
  isLive,
  selected,
  onSelect,
}: {
  version: PromptVersion;
  isLive: boolean;
  selected: boolean;
  onSelect: () => void;
}) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        "w-full text-left px-3.5 py-3 flex gap-2.5 items-start cursor-pointer border-l-[2.5px] border-b border-line",
        selected
          ? "bg-brand-soft border-l-brand"
          : "bg-transparent border-l-transparent hover:bg-bg-sunken",
      )}
    >
      <span
        className={cn(
          "min-w-[34px] text-center px-1 rounded-1 font-mono text-[11px] font-semibold",
          isLive ? "bg-brand text-white" : "bg-bg-sunken text-ink-3",
        )}
      >
        v{version.version}
      </span>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5 text-[12.5px] font-medium text-ink-1">
          <span className="truncate">
            {version.updatedBy?.name ? `@${version.updatedBy.name}` : "—"}
          </span>
          {isLive && (
            <span className="text-[9.5px] px-1.5 rounded-1 bg-success-soft text-success font-semibold uppercase tracking-[0.04em]">
              {t("promptsHistoryLiveBadge")}
            </span>
          )}
        </div>
        <div className="mt-1 text-[11.5px] text-ink-3 font-mono">
          {new Date(version.updatedAt).toLocaleString()}
        </div>
      </div>
    </button>
  );
}

function VersionPill({
  v,
  current = false,
}: {
  v: PromptVersion | null;
  current?: boolean;
}) {
  if (!v) {
    return (
      <span className="px-1.5 py-px rounded-1 font-mono text-[11px] bg-bg-sunken text-ink-4">
        —
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-1.5 px-2 py-px rounded-1 bg-bg-elev border border-line-strong">
      <span
        className={cn(
          "px-1.5 rounded-1 font-mono text-[11px] font-semibold",
          current ? "bg-brand text-white" : "bg-bg-sunken text-ink-2",
        )}
      >
        v{v.version}
      </span>
      <span className="text-[11px] text-ink-3 font-mono">
        {new Date(v.updatedAt).toLocaleDateString()}
      </span>
    </span>
  );
}

function DiffHunkView({ hunk }: { hunk: DiffHunk }) {
  return (
    <div>
      <div className="px-3 py-1 bg-[#f1ece2] text-ink-3 border-y border-line text-[11.5px] font-mono">
        @@ -{hunk.oldStart} +{hunk.newStart} @@
      </div>
      {hunk.lines.map((line, i) => {
        const bg =
          line.op === "add"
            ? "bg-success-soft/40"
            : line.op === "del"
              ? "bg-danger-soft/40"
              : "bg-transparent";
        const fg =
          line.op === "add"
            ? "text-[#0e5c2c]"
            : line.op === "del"
              ? "text-[#7f1d1d]"
              : "text-ink-2";
        const sign = line.op === "add" ? "+" : line.op === "del" ? "−" : " ";
        return (
          <div key={i} className={cn("flex min-h-[1.6em]", bg)}>
            <span className="flex-none w-[40px] px-1 text-right text-ink-5 text-[11px] select-none border-r border-black/[.04]">
              {line.oldNo ?? ""}
            </span>
            <span className="flex-none w-[40px] px-1 text-right text-ink-5 text-[11px] select-none border-r border-black/[.04]">
              {line.newNo ?? ""}
            </span>
            <span
              className={cn(
                "flex-none w-[18px] text-center font-semibold",
                line.op === "add"
                  ? "text-success"
                  : line.op === "del"
                    ? "text-danger"
                    : "text-ink-5",
              )}
            >
              {sign}
            </span>
            <span
              className={cn("flex-1 px-3 py-px whitespace-pre-wrap", fg)}
              style={{ wordBreak: "break-word" }}
            >
              {line.text || " "}
            </span>
          </div>
        );
      })}
    </div>
  );
}
