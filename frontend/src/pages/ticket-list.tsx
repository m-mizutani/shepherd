import { useEffect, useMemo, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { PageShell } from "../components/ui/page-shell";
import { Card } from "../components/ui/card";
import { Icon, type IconName } from "../components/ui/icon";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Skeleton } from "../components/ui/skeleton";
import { ErrorBox } from "../components/ui/error-box";
import { EmptyState } from "../components/ui/empty-state";
import { Popover, PopoverHeader, PopoverItem, PopoverSep } from "../components/ui/popover";
import { Dialog } from "../components/ui/dialog";
import { SlackUserName } from "../components/slack-user-name";
import { cn } from "../lib/utils";

const COLUMN_KEY = (wsId: string) => `shepherd.tickets.cols.${wsId}`;
const SORT_OPTIONS = [
  { value: "updatedAt", label: "Updated", icon: "sort" as IconName },
  { value: "createdAt", label: "Created", icon: "cal" as IconName },
  { value: "seqNum", label: "#", icon: "hash" as IconName },
  { value: "statusId", label: "Status", icon: "flag" as IconName },
] as const;
type SortKey = (typeof SORT_OPTIONS)[number]["value"];
const PAGE_SIZE = 25;

function timeAgo(iso: string): string {
  const d = new Date(iso).getTime();
  const diff = Math.floor((Date.now() - d) / 1000);
  if (diff < 60) return `${diff}s`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)}d`;
  return new Date(iso).toLocaleDateString();
}

export default function TicketListPage() {
  const { workspaceId } = useParams<{ workspaceId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string | null>(null);
  const [closedFilter, setClosedFilter] = useState<"all" | "open" | "closed">("all");
  const [sortKey, setSortKey] = useState<SortKey>("updatedAt");
  const [sortDesc, setSortDesc] = useState(true);
  const [page, setPage] = useState(0);
  const [extraFieldCols, setExtraFieldCols] = useState<string[]>([]);
  const [newTicketOpen, setNewTicketOpen] = useState(false);

  // Load column prefs
  useEffect(() => {
    if (!workspaceId) return;
    const raw = window.localStorage.getItem(COLUMN_KEY(workspaceId));
    if (raw) {
      try {
        const parsed = JSON.parse(raw);
        if (Array.isArray(parsed)) setExtraFieldCols(parsed.filter((x) => typeof x === "string"));
      } catch {
        /* ignore */
      }
    }
  }, [workspaceId]);

  useEffect(() => {
    if (!workspaceId) return;
    window.localStorage.setItem(COLUMN_KEY(workspaceId), JSON.stringify(extraFieldCols));
  }, [workspaceId, extraFieldCols]);

  const { data: configData } = useQuery({
    queryKey: ["workspace-config", workspaceId],
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/config",
        { params: { path: { workspaceId: workspaceId! } } },
      );
      if (error) throw error;
      return data;
    },
    enabled: !!workspaceId,
  });

  const queryParams = useMemo(() => {
    const q: Record<string, string | boolean> = {};
    if (statusFilter) q.statusId = statusFilter;
    if (closedFilter !== "all") q.isClosed = closedFilter === "closed";
    return q;
  }, [statusFilter, closedFilter]);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["tickets", workspaceId, queryParams],
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/tickets",
        {
          params: {
            path: { workspaceId: workspaceId! },
            query: queryParams,
          },
        },
      );
      if (error) throw error;
      return data;
    },
    enabled: !!workspaceId,
  });

  const statusMap = useMemo(
    () => new Map(configData?.statuses?.map((s) => [s.id, s]) ?? []),
    [configData],
  );
  const fieldMap = useMemo(
    () => new Map(configData?.fields?.map((f) => [f.id, f]) ?? []),
    [configData],
  );

  const tickets = data?.tickets ?? [];

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    let arr = tickets;
    if (q) {
      arr = arr.filter((t) => {
        if (`#${t.seqNum}`.includes(q)) return true;
        if (t.title.toLowerCase().includes(q)) return true;
        if ((t.description ?? "").toLowerCase().includes(q)) return true;
        return false;
      });
    }
    arr = [...arr].sort((a, b) => {
      let cmp = 0;
      if (sortKey === "updatedAt") cmp = a.updatedAt.localeCompare(b.updatedAt);
      else if (sortKey === "createdAt") cmp = a.createdAt.localeCompare(b.createdAt);
      else if (sortKey === "seqNum") cmp = a.seqNum - b.seqNum;
      else if (sortKey === "statusId") cmp = a.statusId.localeCompare(b.statusId);
      return sortDesc ? -cmp : cmp;
    });
    return arr;
  }, [tickets, search, sortKey, sortDesc]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  useEffect(() => {
    if (page >= totalPages) setPage(0);
  }, [totalPages, page]);
  const pageStart = page * PAGE_SIZE;
  const pageRows = filtered.slice(pageStart, pageStart + PAGE_SIZE);

  const renderField = (fieldId: string, value: unknown) => {
    const def = fieldMap.get(fieldId);
    if (!def || value === null || value === undefined || value === "")
      return <span className="text-ink-4 text-[12.5px]">—</span>;
    if (def.type === "select") {
      const opt = def.options?.find((o) => o.id === String(value));
      if (opt)
        return (
          <Badge color={opt.color} dot={false}>
            {opt.name}
          </Badge>
        );
      return <span className="text-[12.5px]">{String(value)}</span>;
    }
    if (def.type === "multi-select") {
      const ids = Array.isArray(value) ? value : [value];
      return (
        <div className="flex flex-wrap gap-1">
          {ids.map((v) => {
            const opt = def.options?.find((o) => o.id === String(v));
            return (
              <Badge key={String(v)} color={opt?.color} dot={false}>
                {opt?.name ?? String(v)}
              </Badge>
            );
          })}
        </div>
      );
    }
    if (def.type === "user")
      return <SlackUserName workspaceId={workspaceId!} userId={String(value)} />;
    if (def.type === "date")
      return (
        <span className="text-[12.5px] font-mono text-ink-2">
          {new Date(String(value)).toLocaleDateString()}
        </span>
      );
    if (def.type === "url")
      return (
        <a
          href={String(value)}
          target="_blank"
          rel="noopener noreferrer"
          className="text-info text-[12.5px] underline-offset-2 hover:underline"
        >
          {String(value).replace(/^https?:\/\//, "")}
        </a>
      );
    return <span className="text-[12.5px]">{String(value)}</span>;
  };

  return (
    <PageShell
      crumbs={[{ label: "Tickets" }]}
      showSettings
    >
      <div className="max-w-[1240px] mx-auto px-8 pt-5 pb-10">
        {/* Title row */}
        <div className="flex items-center justify-between mb-3.5">
          <div className="flex items-baseline gap-2.5">
            <h1 className="m-0 text-[22px] font-semibold tracking-[-0.018em] text-ink-1">
              Tickets
            </h1>
            <div className="text-[12px] text-ink-3">
              {!isLoading && `${filtered.length} of ${tickets.length}`}
            </div>
          </div>
          <div className="flex gap-2">
            <label className="inline-flex items-center gap-1.5 h-[30px] px-2.5 bg-bg-elev border border-line-strong rounded-2 text-[13px] w-[260px]">
              <Icon name="search" size={13} className="text-ink-4" />
              <input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search tickets, #IDs…"
                className="flex-1 bg-transparent border-0 outline-none text-ink-1 placeholder:text-ink-4"
              />
              <span className="font-mono text-[10px] px-1 py-px rounded-1 bg-bg-sunken border border-line text-ink-3">
                ⌘K
              </span>
            </label>
            <Button variant="primary" onClick={() => setNewTicketOpen(true)}>
              <Icon name="plus" size={13} /> New ticket
            </Button>
          </div>
        </div>

        {/* Filter bar */}
        <div className="flex items-center gap-1.5 px-2.5 py-2 bg-bg-elev border border-line border-b-0 rounded-t-3 flex-wrap">
          <FilterChip
            icon="filter"
            label="Status"
            value={statusFilter ? statusMap.get(statusFilter)?.name : undefined}
            popover={(close) => (
              <>
                <PopoverHeader>Filter by status</PopoverHeader>
                <PopoverItem
                  active={statusFilter === null}
                  onClick={() => {
                    setStatusFilter(null);
                    close();
                  }}
                >
                  Any status
                </PopoverItem>
                <PopoverSep />
                {configData?.statuses?.map((s) => (
                  <PopoverItem
                    key={s.id}
                    active={statusFilter === s.id}
                    onClick={() => {
                      setStatusFilter(s.id);
                      close();
                    }}
                  >
                    <span
                      className="w-2 h-2 rounded-full flex-none"
                      style={{ background: s.color }}
                    />
                    <span className="flex-1">{s.name}</span>
                  </PopoverItem>
                ))}
              </>
            )}
            onClear={statusFilter ? () => setStatusFilter(null) : undefined}
          />
          <FilterChip
            icon="inbox"
            label="State"
            value={
              closedFilter === "open"
                ? "Open"
                : closedFilter === "closed"
                  ? "Closed"
                  : undefined
            }
            popover={(close) => (
              <>
                <PopoverItem
                  active={closedFilter === "all"}
                  onClick={() => {
                    setClosedFilter("all");
                    close();
                  }}
                >
                  All
                </PopoverItem>
                <PopoverItem
                  active={closedFilter === "open"}
                  onClick={() => {
                    setClosedFilter("open");
                    close();
                  }}
                >
                  Open only
                </PopoverItem>
                <PopoverItem
                  active={closedFilter === "closed"}
                  onClick={() => {
                    setClosedFilter("closed");
                    close();
                  }}
                >
                  Closed only
                </PopoverItem>
              </>
            )}
            onClear={
              closedFilter !== "all" ? () => setClosedFilter("all") : undefined
            }
          />

          <Popover
            trigger={(toggle) => (
              <button
                type="button"
                onClick={toggle}
                className="h-6 inline-flex items-center gap-1 px-2 text-[12px] text-ink-4 hover:text-ink-1"
              >
                <Icon name="plus" size={11} /> Column
              </button>
            )}
          >
            {(close) => (
              <>
                <PopoverHeader>Show field columns</PopoverHeader>
                {configData?.fields?.length ? (
                  configData.fields.map((f) => {
                    const checked = extraFieldCols.includes(f.id);
                    return (
                      <PopoverItem
                        key={f.id}
                        active={checked}
                        onClick={() => {
                          setExtraFieldCols((cur) =>
                            checked ? cur.filter((x) => x !== f.id) : [...cur, f.id],
                          );
                        }}
                      >
                        <span className="w-3.5 inline-flex justify-center text-brand">
                          {checked && <Icon name="check" size={12} />}
                        </span>
                        <span className="flex-1">{f.name}</span>
                        <span className="text-[11px] text-ink-4">{f.type}</span>
                      </PopoverItem>
                    );
                  })
                ) : (
                  <div className="px-2 py-2 text-[12px] text-ink-4">No fields</div>
                )}
                <PopoverSep />
                <PopoverItem onClick={() => { setExtraFieldCols([]); close(); }}>
                  <Icon name="x" size={12} className="text-ink-4" />
                  Clear all
                </PopoverItem>
              </>
            )}
          </Popover>

          <div className="flex-1" />

          <Popover
            align="end"
            trigger={(toggle) => (
              <button
                type="button"
                onClick={toggle}
                className="h-6 inline-flex items-center gap-1.5 px-2 text-[12px] text-ink-2 border border-transparent hover:bg-bg-sunken rounded-1"
              >
                <Icon name="sort" size={12} />{" "}
                {SORT_OPTIONS.find((o) => o.value === sortKey)?.label}{" "}
                {sortDesc ? "↓" : "↑"}
              </button>
            )}
          >
            {(close) => (
              <>
                <PopoverHeader>Sort by</PopoverHeader>
                {SORT_OPTIONS.map((o) => (
                  <PopoverItem
                    key={o.value}
                    active={sortKey === o.value}
                    onClick={() => {
                      setSortKey(o.value);
                      close();
                    }}
                  >
                    <Icon name={o.icon} size={12} className="text-ink-4" />
                    {o.label}
                  </PopoverItem>
                ))}
                <PopoverSep />
                <PopoverItem onClick={() => { setSortDesc(!sortDesc); close(); }}>
                  <Icon name={sortDesc ? "chevron" : "chevronUp"} size={12} className="text-ink-4" />
                  {sortDesc ? "Descending" : "Ascending"}
                </PopoverItem>
              </>
            )}
          </Popover>
        </div>

        {/* Table */}
        <Card className="rounded-t-none border-t-0 p-0 overflow-hidden">
          {isLoading && (
            <div className="p-4 space-y-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="flex gap-4">
                  <Skeleton width={40} />
                  <Skeleton width="40%" />
                  <Skeleton width={100} />
                  <Skeleton width={90} />
                </div>
              ))}
            </div>
          )}
          {error && (
            <div className="p-4">
              <ErrorBox
                title="Failed to load tickets"
                onRetry={() => refetch()}
              />
            </div>
          )}
          {!isLoading && !error && (
            <table className="w-full border-separate border-spacing-0">
              <thead>
                <tr>
                  <Th width={56}>#</Th>
                  <Th>Title</Th>
                  <Th width={160}>Status</Th>
                  {extraFieldCols.map((id) => (
                    <Th key={id} width={140}>
                      {fieldMap.get(id)?.name ?? id}
                    </Th>
                  ))}
                  <Th width={140}>Assignee</Th>
                  <Th width={100} align="right">
                    Updated
                  </Th>
                </tr>
              </thead>
              <tbody>
                {pageRows.map((t) => {
                  const status = statusMap.get(t.statusId);
                  return (
                    <tr
                      key={t.id}
                      onClick={() =>
                        navigate(`/ws/${workspaceId}/tickets/${t.id}`)
                      }
                      className="cursor-pointer hover:bg-[#faf8f3] [&_td]:border-b [&_td]:border-line"
                    >
                      <Td className="font-mono text-ink-4">#{t.seqNum}</Td>
                      <Td>
                        <span className="font-medium text-ink-1">{t.title}</span>
                      </Td>
                      <Td>
                        {status && (
                          <Badge color={status.color}>{status.name}</Badge>
                        )}
                      </Td>
                      {extraFieldCols.map((id) => {
                        const fv = t.fields?.find((f) => f.fieldId === id);
                        return <Td key={id}>{renderField(id, fv?.value)}</Td>;
                      })}
                      <Td>
                        {t.assigneeId ? (
                          <SlackUserName
                            workspaceId={workspaceId!}
                            userId={t.assigneeId}
                          />
                        ) : (
                          <span className="text-ink-4 text-[12.5px] italic">
                            Unassigned
                          </span>
                        )}
                      </Td>
                      <Td className="text-right text-ink-3 text-[12px] font-mono">
                        {timeAgo(t.updatedAt)}
                      </Td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
          {!isLoading && !error && filtered.length === 0 && (
            <EmptyState
              icon="inbox"
              title="No tickets here yet"
              description={
                tickets.length === 0 ? (
                  <>
                    Slack messages posted in tracked channels become tickets
                    automatically.{" "}
                    <Link
                      to={`/ws/${workspaceId}/settings`}
                      className="text-brand-ink underline"
                    >
                      Open settings
                    </Link>{" "}
                    to see which channels are tracked.
                  </>
                ) : (
                  "Try clearing filters or adjusting your search."
                )
              }
            />
          )}
        </Card>

        {/* Pagination */}
        {!isLoading && !error && filtered.length > 0 && (
          <div className="flex items-center justify-between mt-3.5">
            <div className="text-[12px] text-ink-3">
              Showing {pageStart + 1}–{Math.min(pageStart + PAGE_SIZE, filtered.length)} of{" "}
              {filtered.length}
            </div>
            <div className="flex gap-1.5">
              <Button
                size="sm"
                disabled={page === 0}
                onClick={() => setPage((p) => Math.max(0, p - 1))}
              >
                <Icon name="chevronL" size={12} />
              </Button>
              {Array.from({ length: totalPages }).slice(0, 5).map((_, i) => (
                <Button
                  key={i}
                  size="sm"
                  variant={i === page ? "primary" : "default"}
                  onClick={() => setPage(i)}
                >
                  {i + 1}
                </Button>
              ))}
              {totalPages > 5 && (
                <span className="px-1.5 text-ink-4 text-[12px] self-center">
                  …
                </span>
              )}
              <Button
                size="sm"
                disabled={page >= totalPages - 1}
                onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              >
                <Icon name="chevronR" size={12} />
              </Button>
            </div>
          </div>
        )}
      </div>

      <NewTicketDialog
        open={newTicketOpen}
        onClose={() => setNewTicketOpen(false)}
        workspaceId={workspaceId!}
        defaultStatusId={configData?.ticketConfig?.defaultStatusId}
        onCreated={(ticketId) => {
          setNewTicketOpen(false);
          queryClient.invalidateQueries({ queryKey: ["tickets", workspaceId] });
          navigate(`/ws/${workspaceId}/tickets/${ticketId}`);
        }}
      />
    </PageShell>
  );
}

function Th({
  children,
  width,
  align = "left",
}: {
  children: React.ReactNode;
  width?: number;
  align?: "left" | "right";
}) {
  return (
    <th
      style={{ width }}
      className={cn(
        "py-2.5 px-3.5 text-[11.5px] font-medium uppercase tracking-[0.04em] text-ink-4 bg-bg-elev border-b border-line",
        align === "right" ? "text-right" : "text-left",
      )}
    >
      {children}
    </th>
  );
}

function Td({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <td
      className={cn(
        "py-3 px-3.5 text-[13px] align-middle text-ink-1",
        className,
      )}
    >
      {children}
    </td>
  );
}

function FilterChip({
  icon,
  label,
  value,
  popover,
  onClear,
}: {
  icon: IconName;
  label: string;
  value?: string;
  popover: (close: () => void) => React.ReactNode;
  onClear?: () => void;
}) {
  const active = !!value;
  return (
    <Popover
      trigger={(toggle) => (
        <button
          type="button"
          onClick={toggle}
          className={cn(
            "h-6 inline-flex items-center gap-1.5 px-2 text-[12px] rounded-2 border",
            active
              ? "bg-brand-soft border-brand text-brand-ink"
              : "bg-bg-elev border-line-strong text-ink-2 hover:bg-bg-sunken",
          )}
        >
          <Icon name={icon} size={11} />
          <span>
            {label}
            {value && ": "}
            {value && <b className="font-semibold">{value}</b>}
          </span>
          {active && onClear && (
            <span
              role="button"
              tabIndex={0}
              onClick={(e) => {
                e.stopPropagation();
                onClear();
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  e.stopPropagation();
                  onClear();
                }
              }}
              aria-label={`Clear ${label} filter`}
              className="text-ink-3 hover:text-ink-1 ml-0.5"
            >
              <Icon name="x" size={10} />
            </span>
          )}
        </button>
      )}
    >
      {popover}
    </Popover>
  );
}

function NewTicketDialog({
  open,
  onClose,
  workspaceId,
  defaultStatusId,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  workspaceId: string;
  defaultStatusId?: string;
  onCreated: (ticketId: string) => void;
}) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const create = useMutation({
    mutationFn: async () => {
      const { data, error } = await api.POST(
        "/api/v1/ws/{workspaceId}/tickets",
        {
          params: { path: { workspaceId } },
          body: {
            title,
            description: description || undefined,
            statusId: defaultStatusId || undefined,
          },
        },
      );
      if (error) throw error;
      return data!;
    },
    onSuccess: (ticket) => {
      setTitle("");
      setDescription("");
      onCreated(ticket.id);
    },
  });

  useEffect(() => {
    if (!open) {
      setTitle("");
      setDescription("");
      create.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="New ticket"
      width={520}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="primary"
            disabled={!title.trim() || create.isPending}
            onClick={() => create.mutate()}
          >
            {create.isPending ? "Creating…" : "Create ticket"}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <label className="block">
          <span className="text-[12px] font-medium text-ink-3">Title</span>
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            autoFocus
            placeholder="Short summary of the issue"
            className="mt-1 w-full h-9 px-3 bg-bg-elev border border-line-strong rounded-2 text-[13.5px] text-ink-1 focus:outline-none focus:border-brand focus:ring-2 focus:ring-brand-soft"
          />
        </label>
        <label className="block">
          <span className="text-[12px] font-medium text-ink-3">
            Description (optional)
          </span>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={5}
            placeholder="Slack-style markdown is supported."
            className="mt-1 w-full px-3 py-2 bg-bg-elev border border-line-strong rounded-2 text-[13.5px] text-ink-1 focus:outline-none focus:border-brand focus:ring-2 focus:ring-brand-soft"
          />
        </label>
        {create.isError && (
          <ErrorBox title="Failed to create" onRetry={() => create.mutate()} />
        )}
      </div>
    </Dialog>
  );
}
