import { useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearchParams } from "react-router-dom";
import { api } from "../lib/api";
import { Badge } from "../components/ui/badge";
import { Icon, type IconName } from "../components/ui/icon";
import { ErrorBox } from "../components/ui/error-box";
import { Skeleton } from "../components/ui/skeleton";
import { SlackUserName } from "../components/slack-user-name";
import { cn } from "../lib/utils";
import { useTranslation } from "../i18n";
import type { components } from "../generated/api";

type Ticket = components["schemas"]["Ticket"];
type Status = components["schemas"]["StatusDef"];

type GroupBy = "status" | "assignee";

const COLUMN_W = 290;
const LANE_W = 168;
const LANE_GAP = 14;

type DragData = {
  ticketId: string;
  fromStatusId: string;
  // The assignee lane the card was dragged from. Empty string means the
  // unassigned lane. A ticket may render in multiple assignee lanes; this
  // captures whichever one initiated the drag so self-drops are detected.
  fromAssigneeLaneId: string;
};

function timeAgo(iso: string): string {
  const d = new Date(iso).getTime();
  const diff = Math.max(0, Math.floor((Date.now() - d) / 1000));
  if (diff < 60) return `${diff}s`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)}d`;
  return new Date(iso).toLocaleDateString();
}

export function TicketKanbanView({
  workspaceId,
  tickets,
  statuses,
  isLoading,
  error,
  onRetry,
}: {
  workspaceId: string;
  tickets: Ticket[];
  statuses: Status[];
  isLoading: boolean;
  error: unknown;
  onRetry: () => void;
}) {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const urlGroupBy = params.get("groupBy");
  const groupBy: GroupBy = urlGroupBy === "assignee" ? "assignee" : "status";
  const groupByKey = `shepherd.tickets.groupBy.${workspaceId}`;
  const setGroupBy = (g: GroupBy) => {
    const next = new URLSearchParams(params);
    next.set("groupBy", g);
    setParams(next, { replace: true });
  };

  // Restore last-used groupBy when no QS override is present; persist on change.
  useEffect(() => {
    if (urlGroupBy !== null) {
      window.localStorage.setItem(groupByKey, groupBy);
      return;
    }
    const saved = window.localStorage.getItem(groupByKey);
    if (saved === "assignee" || saved === "status") {
      const next = new URLSearchParams(params);
      next.set("groupBy", saved);
      setParams(next, { replace: true });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [urlGroupBy, groupByKey]);

  const queryClient = useQueryClient();
  const [moveError, setMoveError] = useState<string | null>(null);

  useEffect(() => {
    if (!moveError) return;
    const timer = window.setTimeout(() => setMoveError(null), 3500);
    return () => window.clearTimeout(timer);
  }, [moveError]);

  const move = useMutation({
    mutationFn: async (vars: {
      ticketId: string;
      statusId: string;
      assigneeIds?: string[];
      assigneeChange: boolean;
    }) => {
      const body: { statusId?: string; assigneeIds?: string[] } = {
        statusId: vars.statusId,
      };
      if (vars.assigneeChange) body.assigneeIds = vars.assigneeIds ?? [];
      const { data, error } = await api.PATCH(
        "/api/v1/ws/{workspaceId}/tickets/{ticketId}",
        {
          params: {
            path: { workspaceId, ticketId: vars.ticketId },
          },
          body,
        },
      );
      if (error) throw error;
      return data;
    },
    onMutate: async (vars) => {
      await queryClient.cancelQueries({ queryKey: ["tickets", workspaceId] });
      const snapshot = queryClient.getQueriesData({
        queryKey: ["tickets", workspaceId],
      });
      queryClient.setQueriesData(
        { queryKey: ["tickets", workspaceId] },
        (old: { tickets?: Ticket[] } | undefined) => {
          if (!old?.tickets) return old;
          return {
            ...old,
            tickets: old.tickets.map((tk) =>
              tk.id === vars.ticketId
                ? {
                    ...tk,
                    statusId: vars.statusId,
                    ...(vars.assigneeChange
                      ? { assigneeIds: vars.assigneeIds ?? [] }
                      : {}),
                  }
                : tk,
            ),
          };
        },
      );
      return { snapshot };
    },
    onError: (_err, _vars, context) => {
      if (context?.snapshot) {
        for (const [key, value] of context.snapshot) {
          queryClient.setQueryData(key, value);
        }
      }
      setMoveError(t("ticketBoardUpdateFailed"));
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ["tickets", workspaceId] });
    },
  });

  const assignees = useMemo(() => {
    const ids: string[] = [];
    const seen = new Set<string>();
    for (const tk of tickets) {
      for (const id of tk.assigneeIds ?? []) {
        if (!id) continue;
        if (seen.has(id)) continue;
        seen.add(id);
        ids.push(id);
      }
    }
    ids.sort();
    return ids;
  }, [tickets]);

  if (error) {
    return (
      <div className="px-8 py-6">
        <ErrorBox title={t("ticketListLoadFailed")} onRetry={onRetry} />
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col min-h-0 overflow-hidden">
      <div className="px-8 mb-3.5 flex items-center gap-2 flex-wrap">
        <span className="text-[12px] text-ink-3">{t("ticketBoardGroupBy")}</span>
        <div className="inline-flex border border-line-strong rounded-2 overflow-hidden bg-bg-elev">
          <GroupByBtn
            icon="flag"
            label={t("ticketBoardGroupByStatus")}
            active={groupBy === "status"}
            onClick={() => setGroupBy("status")}
          />
          <GroupByBtn
            icon="user"
            label={t("ticketBoardGroupByAssignee")}
            active={groupBy === "assignee"}
            onClick={() => setGroupBy("assignee")}
          />
          <GroupByBtn
            icon="hash"
            label={t("ticketBoardGroupByCategory")}
            disabled
          />
          <GroupByBtn
            icon="cal"
            label={t("ticketBoardGroupByDue")}
            disabled
          />
        </div>
        <div className="flex-1" />
        {moveError && (
          <span className="text-[12px] text-danger" role="alert">
            {moveError}
          </span>
        )}
      </div>

      <div
        className="flex-1 overflow-auto px-8 pb-6"
        data-testid="kanban-scroll"
      >
        {isLoading ? (
          <BoardSkeleton statuses={statuses} />
        ) : groupBy === "status" ? (
          <BoardByStatus
            workspaceId={workspaceId}
            tickets={tickets}
            statuses={statuses}
            onMove={(d, target) =>
              move.mutate({
                ticketId: d.ticketId,
                statusId: target.statusId,
                assigneeChange: false,
              })
            }
          />
        ) : (
          <BoardByAssignee
            workspaceId={workspaceId}
            tickets={tickets}
            statuses={statuses}
            assignees={assignees}
            onMove={(d, target) => {
              const targetIds = target.laneAssigneeId
                ? [target.laneAssigneeId]
                : [];
              const tk = tickets.find((x) => x.id === d.ticketId);
              const before = tk?.assigneeIds ?? [];
              // Compare by membership so the lane-replace drop only fires a
              // PATCH when the assignee set actually differs.
              const beforeSet = new Set(before);
              const targetSet = new Set(targetIds);
              const sameMembership =
                beforeSet.size === targetSet.size &&
                [...beforeSet].every((id) => targetSet.has(id));
              move.mutate({
                ticketId: d.ticketId,
                statusId: target.statusId,
                assigneeIds: targetIds,
                assigneeChange: !sameMembership,
              });
            }}
          />
        )}
      </div>
    </div>
  );
}

function GroupByBtn({
  icon,
  label,
  active,
  disabled,
  onClick,
}: {
  icon: IconName;
  label: string;
  active?: boolean;
  disabled?: boolean;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "h-7 px-3 inline-flex items-center gap-1.5 text-[12.5px] border-r border-line last:border-r-0",
        active
          ? "bg-brand-soft text-brand-ink font-semibold"
          : disabled
            ? "text-ink-5 cursor-not-allowed"
            : "text-ink-2 hover:bg-bg-sunken font-medium",
      )}
      aria-pressed={active}
    >
      <Icon name={icon} size={12} />
      {label}
    </button>
  );
}

// ── Status board ────────────────────────────────────
function BoardByStatus({
  workspaceId,
  tickets,
  statuses,
  onMove,
}: {
  workspaceId: string;
  tickets: Ticket[];
  statuses: Status[];
  onMove: (d: DragData, target: { statusId: string }) => void;
}) {
  return (
    <div className="flex gap-3.5 h-full items-start">
      {statuses.map((s) => {
        const cards = tickets.filter((t) => t.statusId === s.id);
        return (
          <KanbanColumn
            key={s.id}
            accent={s.color}
            header={
              <Badge color={s.color} size="md">
                {s.name}
              </Badge>
            }
            count={cards.length}
            onDropTicket={(d) => {
              if (d.fromStatusId === s.id) return;
              onMove(d, { statusId: s.id });
            }}
          >
            {cards.length === 0 ? (
              <ColumnEmpty />
            ) : (
              cards.map((t) => (
                <TicketCard
                  key={t.id}
                  workspaceId={workspaceId}
                  ticket={t}
                  showAssignee
                />
              ))
            )}
          </KanbanColumn>
        );
      })}
    </div>
  );
}

// ── Assignee swimlanes ──────────────────────────────
//
// Tickets with multiple assignees render once per lane they belong to.
// Dropping a card onto a lane *replaces* its assignee list with that lane's
// owner (or empties it for the Unassigned lane).
function BoardByAssignee({
  workspaceId,
  tickets,
  statuses,
  assignees,
  onMove,
}: {
  workspaceId: string;
  tickets: Ticket[];
  statuses: Status[];
  assignees: string[];
  onMove: (
    d: DragData,
    target: { statusId: string; laneAssigneeId: string },
  ) => void;
}) {
  const { t } = useTranslation();
  const lanes: Array<{ id: string; isUnassigned: boolean }> = [
    ...assignees.map((id) => ({ id, isUnassigned: false })),
    { id: "", isUnassigned: true },
  ];

  return (
    <div className="inline-flex flex-col min-w-full">
      {/* Sticky column header row */}
      <div
        className="sticky top-0 z-[3] flex gap-3.5 pb-2"
        style={{
          marginLeft: LANE_W,
          background: "linear-gradient(var(--bg) 80%, transparent)",
        }}
      >
        {statuses.map((s) => {
          const total = tickets.filter((tk) => tk.statusId === s.id).length;
          return (
            <div
              key={s.id}
              className="rounded-2 bg-bg-elev border border-line flex items-center gap-2"
              style={{
                flex: `0 0 ${COLUMN_W}px`,
                padding: "8px 10px",
                borderTop: `3px solid ${s.color}`,
              }}
            >
              <div className="flex-1 min-w-0">
                <Badge color={s.color}>{s.name}</Badge>
              </div>
              <span className="text-[11px] font-semibold text-ink-3">
                {total}
              </span>
            </div>
          );
        })}
      </div>

      {lanes.map((lane) => {
        const myCards = tickets.filter((tk) =>
          lane.isUnassigned
            ? !tk.assigneeIds || tk.assigneeIds.length === 0
            : (tk.assigneeIds ?? []).includes(lane.id),
        );
        return (
          <div
            key={lane.id || "unassigned"}
            className={cn(
              "flex border-t border-line",
              lane.isUnassigned && "bg-[rgba(180,170,160,0.04)]",
            )}
          >
            <div
              className="sticky left-0 z-[2] bg-bg border-r border-line flex flex-col gap-1.5"
              style={{
                flex: `0 0 ${LANE_W}px`,
                padding: `${LANE_GAP}px ${LANE_GAP}px ${LANE_GAP}px 4px`,
              }}
            >
              {lane.isUnassigned ? (
                <span className="inline-flex items-center gap-2 text-ink-3">
                  <span className="w-6 h-6 rounded-1 border-[1.5px] border-dashed border-line-strong inline-flex items-center justify-center">
                    <Icon name="user" size={12} className="text-ink-4" />
                  </span>
                  <span className="text-[13px] font-medium">
                    {t("ticketBoardUnassigned")}
                  </span>
                </span>
              ) : (
                <SlackLane workspaceId={workspaceId} userId={lane.id} />
              )}
              <div className="ml-8 inline-flex items-center gap-1.5">
                <span className="text-[10.5px] font-semibold px-1.5 py-px rounded-full bg-bg-sunken text-ink-3">
                  {t("ticketBoardLaneOpen", { count: myCards.length })}
                </span>
              </div>
            </div>

            <div
              className="flex gap-3.5 flex-1"
              style={{ padding: "10px 0" }}
            >
              {statuses.map((s) => {
                const cards = myCards.filter((tk) => tk.statusId === s.id);
                return (
                  <DropCell
                    key={s.id}
                    hasCards={cards.length > 0}
                    onDropTicket={(d) => {
                      if (
                        d.fromStatusId === s.id &&
                        d.fromAssigneeLaneId === lane.id
                      )
                        return;
                      onMove(d, {
                        statusId: s.id,
                        laneAssigneeId: lane.id,
                      });
                    }}
                  >
                    {cards.length === 0 ? (
                      <div className="flex-1 min-h-[76px] border border-dashed border-line rounded-1 flex items-center justify-center text-ink-5 text-[11px]">
                        {t("ticketBoardEmptyCell")}
                      </div>
                    ) : (
                      cards.map((tk) => (
                        <TicketCard
                          key={`${lane.id || "unassigned"}:${tk.id}`}
                          workspaceId={workspaceId}
                          ticket={tk}
                          fromAssigneeLaneId={lane.id}
                        />
                      ))
                    )}
                  </DropCell>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}

function SlackLane({
  workspaceId,
  userId,
}: {
  workspaceId: string;
  userId: string;
}) {
  return (
    <span className="inline-flex items-center gap-2">
      <SlackUserName
        workspaceId={workspaceId}
        userId={userId}
        size="md"
        showAvatar
      />
    </span>
  );
}

// ── Column shell ────────────────────────────────────
function KanbanColumn({
  header,
  count,
  children,
  accent,
  onDropTicket,
}: {
  header: React.ReactNode;
  count: number;
  children: React.ReactNode;
  accent?: string;
  onDropTicket: (d: DragData) => void;
}) {
  const [over, setOver] = useState(false);
  return (
    <div
      className={cn(
        "flex-none w-[290px] max-h-full flex flex-col bg-bg-sunken border border-line rounded-3 overflow-hidden",
        over && "ring-2 ring-brand",
      )}
      onDragOver={(e) => {
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        if (!over) setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(e) => {
        e.preventDefault();
        setOver(false);
        const data = readDrag(e);
        if (data) onDropTicket(data);
      }}
      data-testid="kanban-column"
    >
      {accent && <div style={{ height: 3, background: accent }} />}
      <div className="px-3 py-2.5 flex items-center gap-2 border-b border-line bg-bg-elev">
        <div className="flex-1 min-w-0">{header}</div>
        <span className="text-[11.5px] font-semibold text-ink-3 px-1.5 py-px rounded-full bg-bg-sunken">
          {count}
        </span>
      </div>
      <div className="p-2 flex flex-col gap-2 overflow-y-auto min-h-[200px]">
        {children}
      </div>
    </div>
  );
}

function DropCell({
  hasCards,
  onDropTicket,
  children,
}: {
  hasCards: boolean;
  onDropTicket: (d: DragData) => void;
  children: React.ReactNode;
}) {
  const [over, setOver] = useState(false);
  return (
    <div
      className={cn(
        "rounded-2 p-2 flex flex-col gap-2",
        hasCards
          ? "bg-bg-sunken border border-line"
          : "border border-dashed border-transparent",
        over && "ring-2 ring-brand bg-bg-sunken",
      )}
      style={{
        flex: `0 0 ${COLUMN_W}px`,
        minHeight: 96,
      }}
      onDragOver={(e) => {
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        if (!over) setOver(true);
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(e) => {
        e.preventDefault();
        setOver(false);
        const data = readDrag(e);
        if (data) onDropTicket(data);
      }}
      data-testid="kanban-cell"
    >
      {children}
    </div>
  );
}

function ColumnEmpty() {
  const { t } = useTranslation();
  return (
    <div className="px-2.5 py-4 text-center text-ink-4 text-[12px]">
      {t("ticketBoardColumnEmpty")}
    </div>
  );
}

const DRAG_MIME = "application/x-shepherd-ticket";

function readDrag(e: React.DragEvent): DragData | null {
  try {
    const raw = e.dataTransfer.getData(DRAG_MIME);
    if (!raw) return null;
    return JSON.parse(raw) as DragData;
  } catch {
    return null;
  }
}

// ── Card ────────────────────────────────────────────
function TicketCard({
  workspaceId,
  ticket,
  showAssignee,
  fromAssigneeLaneId,
}: {
  workspaceId: string;
  ticket: Ticket;
  showAssignee?: boolean;
  // When rendered inside an assignee lane, identifies which lane the drag
  // originates from (empty string = Unassigned). Status-grouped boards leave
  // this undefined; we fall back to the ticket's first assignee.
  fromAssigneeLaneId?: string;
}) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const draggingRef = useRef(false);

  return (
    <div
      draggable
      onDragStart={(e) => {
        draggingRef.current = true;
        const data: DragData = {
          ticketId: ticket.id,
          fromStatusId: ticket.statusId,
          fromAssigneeLaneId:
            fromAssigneeLaneId !== undefined
              ? fromAssigneeLaneId
              : (ticket.assigneeIds ?? [])[0] ?? "",
        };
        e.dataTransfer.setData(DRAG_MIME, JSON.stringify(data));
        e.dataTransfer.effectAllowed = "move";
      }}
      onDragEnd={() => {
        // Defer click reset to next tick so onClick (if any) sees dragging
        window.setTimeout(() => {
          draggingRef.current = false;
        }, 0);
      }}
      onClick={() => {
        if (draggingRef.current) return;
        navigate(`/ws/${workspaceId}/tickets/${ticket.id}`);
      }}
      className="group bg-bg-elev border border-line rounded-2 p-2.5 shadow-sm cursor-grab active:cursor-grabbing flex flex-col gap-2 hover:border-line-strong"
      data-testid="kanban-card"
      data-ticket-id={ticket.id}
    >
      <div className="flex items-center gap-1.5">
        <span className="font-mono text-[11px] text-ink-4">
          #{ticket.seqNum}
        </span>
        <div className="flex-1" />
        <span className="text-[11px] text-ink-3 font-mono">
          {timeAgo(ticket.updatedAt)}
        </span>
      </div>
      <div className="text-[13px] font-medium text-ink-1 leading-snug line-clamp-3">
        {ticket.title}
      </div>
      {(ticket.reporterSlackUserId || showAssignee) && (
        <div className="flex flex-col gap-1 mt-0.5 text-[11.5px]">
          {ticket.reporterSlackUserId && (
            <div className="flex items-center gap-1.5 min-w-0">
              <span className="w-8 shrink-0 text-[10.5px] font-medium text-ink-4 uppercase tracking-wide">
                {t("ticketBoardReporterPrefix")}
              </span>
              <span className="min-w-0 truncate">
                <SlackUserName
                  workspaceId={workspaceId}
                  userId={ticket.reporterSlackUserId}
                  size="xs"
                  mute
                />
              </span>
            </div>
          )}
          {showAssignee && (
            <div className="flex items-center gap-1.5 min-w-0">
              <span className="w-8 shrink-0 text-[10.5px] font-medium text-ink-4 uppercase tracking-wide">
                {t("ticketBoardAssigneePrefix")}
              </span>
              {ticket.assigneeIds && ticket.assigneeIds.length > 0 ? (
                <span className="min-w-0 truncate inline-flex items-center gap-1.5">
                  <SlackUserName
                    workspaceId={workspaceId}
                    userId={ticket.assigneeIds[0]}
                    size="xs"
                  />
                  {ticket.assigneeIds.length > 1 && (
                    <span className="text-[10.5px] text-ink-3">
                      {t("ticketAssigneePlusMore", {
                        count: ticket.assigneeIds.length - 1,
                      })}
                    </span>
                  )}
                </span>
              ) : (
                <span
                  title={t("ticketBoardUnassigned")}
                  className="inline-flex items-center gap-1 text-ink-4"
                >
                  <span className="w-4 h-4 rounded-1 border-[1.5px] border-dashed border-line-strong inline-flex items-center justify-center">
                    <Icon name="user" size={9} className="text-ink-4" />
                  </span>
                  <span className="text-[11.5px]">
                    {t("ticketBoardUnassigned")}
                  </span>
                </span>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ── Skeleton loader ─────────────────────────────────
function BoardSkeleton({ statuses }: { statuses: Status[] }) {
  return (
    <div className="flex gap-3.5 h-full items-start">
      {statuses.slice(0, 4).map((s) => (
        <div
          key={s.id}
          className="flex-none w-[290px] flex flex-col bg-bg-sunken border border-line rounded-3 p-2 gap-2"
        >
          <Skeleton width="60%" height={14} />
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} width="100%" height={64} />
          ))}
        </div>
      ))}
    </div>
  );
}

