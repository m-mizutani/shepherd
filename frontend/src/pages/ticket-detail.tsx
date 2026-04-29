import { useState, useMemo, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { api } from "../lib/api";
import { PageShell } from "../components/ui/page-shell";
import { Card } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { Icon } from "../components/ui/icon";
import { Skeleton } from "../components/ui/skeleton";
import { ErrorBox } from "../components/ui/error-box";
import { Avatar } from "../components/ui/avatar";
import { Popover, PopoverItem } from "../components/ui/popover";
import { SlackUserName } from "../components/slack-user-name";
import { SlackMarkdown } from "../components/slack-markdown";
import { UserPicker } from "../components/user-picker";
import { slackThreadUrl } from "../lib/slack";
import { cn } from "../lib/utils";
import { useTranslation } from "../i18n";

const ALLOWED_URL_PROTOCOLS = ["http:", "https:", "mailto:", "ssh:", "ftp:", "ftps:"];

function isValidURL(s: string): boolean {
  try {
    const url = new URL(s);
    return ALLOWED_URL_PROTOCOLS.includes(url.protocol);
  } catch {
    return false;
  }
}

export default function TicketDetailPage() {
  const { workspaceId, ticketId } = useParams<{
    workspaceId: string;
    ticketId: string;
  }>();
  const queryClient = useQueryClient();
  const { t } = useTranslation();

  const [isEditing, setIsEditing] = useState(false);
  const [editTitle, setEditTitle] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editFields, setEditFields] = useState<Record<string, unknown>>({});
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

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
  });

  const { data: slackUsersData } = useQuery({
    queryKey: ["slack-users", workspaceId],
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/slack/users",
        { params: { path: { workspaceId: workspaceId! } } },
      );
      if (error) throw error;
      return data;
    },
    staleTime: 5 * 60 * 1000,
  });

  const {
    data: ticket,
    isLoading,
    error,
    refetch,
  } = useQuery({
    queryKey: ["ticket", workspaceId, ticketId],
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/tickets/{ticketId}",
        {
          params: {
            path: { workspaceId: workspaceId!, ticketId: ticketId! },
          },
        },
      );
      if (error) throw error;
      return data;
    },
  });

  const { data: commentsData } = useQuery({
    queryKey: ["comments", workspaceId, ticketId],
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/tickets/{ticketId}/comments",
        {
          params: {
            path: { workspaceId: workspaceId!, ticketId: ticketId! },
          },
        },
      );
      if (error) throw error;
      return data;
    },
  });

  const updateTicket = useMutation({
    mutationFn: async (body: {
      title?: string;
      description?: string;
      statusId?: string;
      assigneeIds?: string[];
      fields?: { fieldId: string; value: unknown }[];
    }) => {
      const { data, error } = await api.PATCH(
        "/api/v1/ws/{workspaceId}/tickets/{ticketId}",
        {
          params: {
            path: { workspaceId: workspaceId!, ticketId: ticketId! },
          },
          body,
        },
      );
      if (error) throw error;
      return data;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: ["ticket", workspaceId, ticketId],
      });
      const isBulkSave =
        variables.title !== undefined ||
        variables.description !== undefined ||
        variables.fields !== undefined;
      if (isBulkSave) setIsEditing(false);
    },
  });

  const statusMap = useMemo(
    () => new Map(configData?.statuses?.map((s) => [s.id, s]) ?? []),
    [configData],
  );
  const fieldMap = useMemo(
    () => new Map(configData?.fields?.map((f) => [f.id, f]) ?? []),
    [configData],
  );
  const currentStatus = ticket ? statusMap.get(ticket.statusId) : null;
  const threadUrl = slackThreadUrl(ticket?.slackChannelId, ticket?.slackThreadTs);

  const startEditing = () => {
    if (!ticket) return;
    setEditTitle(ticket.title);
    setEditDescription(ticket.description ?? "");
    const fields: Record<string, unknown> = {};
    for (const f of ticket.fields ?? []) fields[f.fieldId] = f.value;
    setEditFields(fields);
    setFieldErrors({});
    setIsEditing(true);
  };

  const saveEdits = () => {
    const errors: Record<string, string> = {};
    for (const [fieldId, value] of Object.entries(editFields)) {
      const def = fieldMap.get(fieldId);
      if (def?.type === "url" && value && !isValidURL(String(value))) {
        errors[fieldId] = t("ticketDetailUrlInvalid");
      }
    }
    if (Object.keys(errors).length > 0) {
      setFieldErrors(errors);
      return;
    }
    const fieldValues = Object.entries(editFields).map(([fieldId, value]) => ({
      fieldId,
      value,
    }));
    updateTicket.mutate({
      title: editTitle,
      description: editDescription,
      fields: fieldValues,
    });
  };

  const renderFieldValue = (fieldId: string, value: unknown): ReactNode => {
    const def = fieldMap.get(fieldId);
    if (!def) return String(value ?? "");
    if (value === null || value === undefined || value === "")
      return <span className="text-ink-4">—</span>;

    switch (def.type) {
      case "url":
        return (
          <a
            href={String(value)}
            target="_blank"
            rel="noopener noreferrer"
            className="text-info text-[12.5px] break-all underline-offset-2 hover:underline"
          >
            {String(value)}
          </a>
        );
      case "user":
        return (
          <SlackUserName workspaceId={workspaceId!} userId={String(value)} />
        );
      case "multi-user": {
        const ids = (Array.isArray(value) ? value : [value]).filter(
          (v) => v !== null && v !== undefined && v !== "",
        );
        return (
          <div className="flex flex-col gap-1">
            {ids.map((id) => (
              <SlackUserName
                key={String(id)}
                workspaceId={workspaceId!}
                userId={String(id)}
              />
            ))}
          </div>
        );
      }
      case "select": {
        const opt = def.options?.find((o) => o.id === String(value));
        if (opt)
          return (
            <Badge color={opt.color} dot={false}>
              {opt.name}
            </Badge>
          );
        return <span className="text-[13px]">{String(value)}</span>;
      }
      case "multi-select": {
        const values = Array.isArray(value) ? value : [value];
        return (
          <div className="flex flex-wrap gap-1">
            {values.map((v) => {
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
      case "date":
        return (
          <span className="text-[12.5px] font-mono text-ink-2">
            {new Date(String(value)).toLocaleDateString()}
          </span>
        );
      default:
        return <span className="text-[13px] text-ink-1">{String(value)}</span>;
    }
  };

  const renderFieldEditor = (fieldId: string) => {
    const def = fieldMap.get(fieldId);
    if (!def) return null;
    const value = editFields[fieldId] ?? "";

    switch (def.type) {
      case "select":
        return (
          <select
            value={String(value)}
            onChange={(e) =>
              setEditFields({ ...editFields, [fieldId]: e.target.value })
            }
            className="w-full h-8 px-2 border border-line-strong bg-bg-elev rounded-2 text-[13px]"
          >
            <option value="">—</option>
            {def.options?.map((opt) => (
              <option key={opt.id} value={opt.id}>
                {opt.name}
              </option>
            ))}
          </select>
        );
      case "multi-select":
        return (
          <select
            multiple
            value={Array.isArray(value) ? value.map(String) : []}
            onChange={(e) => {
              const selected = Array.from(
                e.target.selectedOptions,
                (o) => o.value,
              );
              setEditFields({ ...editFields, [fieldId]: selected });
            }}
            className="w-full px-2 py-1 border border-line-strong bg-bg-elev rounded-2 text-[13px]"
          >
            {def.options?.map((opt) => (
              <option key={opt.id} value={opt.id}>
                {opt.name}
              </option>
            ))}
          </select>
        );
      case "number":
        return (
          <input
            type="number"
            value={String(value)}
            onChange={(e) =>
              setEditFields({
                ...editFields,
                [fieldId]: Number.isNaN(e.target.valueAsNumber)
                  ? ""
                  : e.target.valueAsNumber,
              })
            }
            className="w-full h-8 px-2 border border-line-strong bg-bg-elev rounded-2 text-[13px]"
          />
        );
      case "date":
        return (
          <input
            type="date"
            value={String(value)}
            onChange={(e) =>
              setEditFields({ ...editFields, [fieldId]: e.target.value })
            }
            className="w-full h-8 px-2 border border-line-strong bg-bg-elev rounded-2 text-[13px]"
          />
        );
      case "url":
        return (
          <div>
            <input
              type="text"
              value={String(value)}
              onChange={(e) => {
                setEditFields({ ...editFields, [fieldId]: e.target.value });
                if (e.target.value && !isValidURL(e.target.value)) {
                  setFieldErrors({ ...fieldErrors, [fieldId]: t("ticketDetailUrlInvalid") });
                } else {
                  const { [fieldId]: _, ...rest } = fieldErrors;
                  setFieldErrors(rest);
                }
              }}
              placeholder="https://example.com"
              className={cn(
                "w-full h-8 px-2 border bg-bg-elev rounded-2 text-[13px]",
                fieldErrors[fieldId]
                  ? "border-danger"
                  : "border-line-strong",
              )}
            />
            {fieldErrors[fieldId] && (
              <p className="text-[11px] text-danger mt-0.5">
                {fieldErrors[fieldId]}
              </p>
            )}
          </div>
        );
      case "user":
        return (
          <UserPicker
            users={slackUsersData?.users ?? []}
            value={typeof value === "string" ? value : ""}
            onChange={(v) => setEditFields({ ...editFields, [fieldId]: v })}
          />
        );
      case "multi-user": {
        const arr = Array.isArray(value)
          ? value.map(String)
          : value
            ? [String(value)]
            : [];
        return (
          <UserPicker
            multi
            users={slackUsersData?.users ?? []}
            value={arr}
            onChange={(v) => setEditFields({ ...editFields, [fieldId]: v })}
            placeholder={t("ticketDetailUserPickerSelectUsers")}
          />
        );
      }
      default:
        return (
          <input
            type="text"
            value={String(value)}
            onChange={(e) =>
              setEditFields({ ...editFields, [fieldId]: e.target.value })
            }
            className="w-full h-8 px-2 border border-line-strong bg-bg-elev rounded-2 text-[13px]"
          />
        );
    }
  };

  return (
    <PageShell
      crumbs={[
        { label: t("ticketListCrumb"), to: `/ws/${workspaceId}/tickets` },
        { label: ticket ? `#${ticket.seqNum}` : "" },
      ]}
      showSettings
    >
      <div className="max-w-[1240px] mx-auto px-8 pt-5 pb-24">
        {isLoading && (
          <div className="space-y-3">
            <Skeleton width={120} />
            <Skeleton width="50%" height={24} />
            <Skeleton width="80%" />
            <Skeleton width="70%" />
          </div>
        )}
        {error && (
          <ErrorBox
            title={t("ticketDetailLoadFailed")}
            onRetry={() => refetch()}
          />
        )}

        {ticket && (
          <div className="grid grid-cols-[1fr_320px] gap-7">
            {/* Main */}
            <div>
              {/* Header row */}
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2.5">
                  <span className="font-mono text-[13px] text-ink-4">
                    #{ticket.seqNum}
                  </span>
                  {currentStatus && (
                    <Badge color={currentStatus.color} size="md">
                      {currentStatus.name}
                    </Badge>
                  )}
                </div>
                <div className="flex gap-1.5">
                  {threadUrl && (
                    <a href={threadUrl} target="_blank" rel="noopener noreferrer">
                      <Button size="sm" variant="default" type="button">
                        <Icon name="slack" size={12} /> {t("ticketDetailOpenThread")}
                      </Button>
                    </a>
                  )}
                  {!isEditing && (
                    <Button size="sm" onClick={startEditing}>
                      <Icon name="edit" size={12} /> {t("ticketDetailEdit")}
                    </Button>
                  )}
                </div>
              </div>

              {/* Title */}
              {isEditing ? (
                <input
                  value={editTitle}
                  onChange={(e) => setEditTitle(e.target.value)}
                  className="mt-3.5 w-full text-[24px] font-semibold tracking-[-0.02em] text-ink-1 bg-bg-elev border border-line-strong rounded-2 px-3 py-1.5 focus:outline-none focus:border-brand focus:ring-2 focus:ring-brand-soft"
                />
              ) : (
                <h1 className="mt-3.5 mb-1.5 text-[24px] font-semibold tracking-[-0.02em] text-ink-1">
                  {ticket.title}
                </h1>
              )}

              <div className="text-[12px] text-ink-3 mb-4">
                {ticket.reporterSlackUserId && (
                  <>
                    {t("ticketDetailOpenedBy")}{" "}
                    <span className="text-ink-2 font-medium">
                      <SlackUserName
                        workspaceId={workspaceId!}
                        userId={ticket.reporterSlackUserId}
                        showAvatar={false}
                      />
                    </span>{" "}
                    ·{" "}
                  </>
                )}
                {t("ticketDetailMetadataLine", {
                  created: new Date(ticket.createdAt).toLocaleString(),
                  updated: new Date(ticket.updatedAt).toLocaleString(),
                })}
              </div>

              {/* Description */}
              {isEditing ? (
                <div>
                  <label className="block text-[12px] font-medium text-ink-3 mb-1">
                    {t("ticketDetailDescriptionLabel")}
                  </label>
                  <textarea
                    value={editDescription}
                    onChange={(e) => setEditDescription(e.target.value)}
                    rows={8}
                    className="w-full px-3 py-2 bg-bg-elev border border-line-strong rounded-3 text-[13.5px] text-ink-1 focus:outline-none focus:border-brand focus:ring-2 focus:ring-brand-soft"
                  />
                </div>
              ) : ticket.description ? (
                <Card className="p-4">
                  <SlackMarkdown text={ticket.description} />
                </Card>
              ) : (
                <div className="text-[12.5px] text-ink-4 italic">
                  {t("ticketDetailNoDescription")}
                </div>
              )}

              {/* Activity */}
              <div className="mt-7 flex items-center gap-2">
                <h2 className="m-0 text-[14px] font-semibold text-ink-1">
                  {t("ticketDetailActivity")}
                </h2>
                <span className="text-[12px] text-ink-3">·</span>
                <span className="text-[12px] text-ink-3">
                  {t("ticketDetailEvents", {
                    count: commentsData?.comments?.length ?? 0,
                  })}
                </span>
              </div>

              <div className="mt-3.5 relative pl-3.5">
                <div className="absolute left-[11px] top-5 bottom-7 w-px bg-line" />
                {commentsData?.comments?.length ? (
                  commentsData.comments.map((c) => (
                    <CommentRow
                      key={c.id}
                      workspaceId={workspaceId!}
                      slackUserId={c.slackUserId}
                      time={new Date(c.createdAt).toLocaleString()}
                      body={c.body}
                    />
                  ))
                ) : (
                  <div className="pl-7 py-2 text-[12.5px] text-ink-4">
                    {t("ticketDetailNoComments")}
                  </div>
                )}
              </div>

              {/* Slack reply hint */}
              <div className="mt-4 p-3.5 bg-bg-elev border border-dashed border-line-strong rounded-4 flex items-center gap-3">
                <div className="w-8 h-8 rounded-3 flex items-center justify-center bg-[#ECB22E22] text-[#A37B00]">
                  <Icon name="slack" size={16} />
                </div>
                <div className="flex-1">
                  <div className="text-[13px] font-medium text-ink-1">
                    {t("ticketDetailReplyHintTitle")}
                  </div>
                  <div className="text-[12px] text-ink-3 mt-0.5">
                    {t("ticketDetailReplyHintBody")}
                  </div>
                </div>
                {threadUrl && (
                  <a href={threadUrl} target="_blank" rel="noopener noreferrer">
                    <Button variant="primary" size="sm" type="button">
                      <Icon name="slack" size={12} /> {t("ticketDetailOpenThread")}{" "}
                      <Icon name="arrow" size={11} />
                    </Button>
                  </a>
                )}
              </div>
            </div>

            {/* Unified sidebar */}
            <UnifiedSidebar
              ticket={ticket}
              currentStatus={currentStatus}
              statuses={configData?.statuses ?? []}
              fields={configData?.fields ?? []}
              onChangeStatus={(statusId) => updateTicket.mutate({ statusId })}
              onChangeAssignees={(assigneeIds) =>
                updateTicket.mutate({ assigneeIds })
              }
              isEditing={isEditing}
              isAssigneePending={updateTicket.isPending}
              renderFieldValue={renderFieldValue}
              renderFieldEditor={renderFieldEditor}
              workspaceId={workspaceId!}
              slackUsers={slackUsersData?.users ?? []}
            />
          </div>
        )}
      </div>

      {/* Sticky save bar */}
      {isEditing && ticket && (
        <div className="fixed bottom-0 left-0 right-0 z-20 bg-bg-elev border-t border-line shadow-pop">
          <div className="max-w-[1240px] mx-auto px-8 py-3 flex items-center justify-end gap-2">
            {updateTicket.isError && (
              <span className="text-[12.5px] text-danger mr-auto">
                {t("ticketDetailSaveFailed")}
              </span>
            )}
            <Button variant="ghost" onClick={() => setIsEditing(false)}>
              {t("ticketDetailCancel")}
            </Button>
            <Button
              variant="primary"
              onClick={saveEdits}
              disabled={updateTicket.isPending}
            >
              {updateTicket.isPending
                ? t("ticketDetailSaving")
                : t("ticketDetailSaveChanges")}
            </Button>
          </div>
        </div>
      )}
    </PageShell>
  );
}

function CommentRow({
  workspaceId,
  slackUserId,
  time,
  body,
}: {
  workspaceId: string;
  slackUserId: string;
  time: string;
  body: string;
}) {
  return (
    <div className="relative mb-3.5 pl-7">
      <div className="absolute -left-1 top-0">
        <Avatar name={slackUserId} size="md" />
      </div>
      <Card className="px-3.5 py-2.5 shadow-none">
        <div className="flex items-baseline gap-2 mb-1">
          <SlackUserName
            workspaceId={workspaceId}
            userId={slackUserId}
            showAvatar={false}
          />
          <span className="text-[12px] text-ink-3">{time}</span>
        </div>
        <SlackMarkdown text={body} />
      </Card>
    </div>
  );
}

interface UnifiedSidebarProps {
  ticket: {
    id: string;
    statusId: string;
    assigneeIds: string[];
    reporterSlackUserId?: string;
    slackChannelId?: string;
    fields?: { fieldId: string; value: unknown }[];
    createdAt: string;
    updatedAt: string;
  };
  currentStatus: { id: string; name: string; color: string } | null | undefined;
  statuses: { id: string; name: string; color: string }[];
  fields: { id: string; name: string; type: string; required: boolean }[];
  onChangeStatus: (id: string) => void;
  onChangeAssignees: (ids: string[]) => void;
  isEditing: boolean;
  isAssigneePending: boolean;
  renderFieldValue: (fieldId: string, value: unknown) => ReactNode;
  renderFieldEditor: (fieldId: string) => ReactNode;
  workspaceId: string;
  slackUsers: { id: string; name: string; email?: string; imageUrl?: string }[];
}

function UnifiedSidebar({
  ticket,
  currentStatus,
  statuses,
  fields,
  onChangeStatus,
  onChangeAssignees,
  isEditing,
  isAssigneePending,
  renderFieldValue,
  renderFieldEditor,
  workspaceId,
  slackUsers,
}: UnifiedSidebarProps) {
  const { t } = useTranslation();
  return (
    <Card className="self-start sticky top-4 p-0 overflow-hidden">
      <div className="px-4 py-3.5 border-b border-line">
        <div className="text-[11px] font-semibold uppercase tracking-[0.05em] text-ink-3 mb-1.5">
          {t("ticketDetailLabelStatus")}
        </div>
        <Popover
          trigger={(toggle) => (
            <button
              type="button"
              onClick={toggle}
              className="w-full flex items-center justify-between gap-1.5 px-2.5 py-2 rounded-3 cursor-pointer border"
              style={
                currentStatus
                  ? {
                      background: currentStatus.color + "15",
                      borderColor: currentStatus.color + "40",
                    }
                  : { borderColor: "var(--line-strong)" }
              }
            >
              {currentStatus ? (
                <Badge color={currentStatus.color}>{currentStatus.name}</Badge>
              ) : (
                <span className="text-[13px] text-ink-3">{ticket.statusId}</span>
              )}
              <Icon name="chevron" size={13} className="text-ink-4" />
            </button>
          )}
        >
          {(close) => (
            <>
              {statuses.map((s) => (
                <PopoverItem
                  key={s.id}
                  active={s.id === ticket.statusId}
                  onClick={() => {
                    if (s.id !== ticket.statusId) onChangeStatus(s.id);
                    close();
                  }}
                >
                  <span
                    className="w-2.5 h-2.5 rounded-full flex-none"
                    style={{ background: s.color }}
                  />
                  <span className="flex-1">{s.name}</span>
                  {s.id === ticket.statusId && (
                    <Icon name="check" size={12} className="text-brand" />
                  )}
                </PopoverItem>
              ))}
            </>
          )}
        </Popover>
      </div>

      <div className="px-4 py-3.5 flex flex-col gap-3">
        {fields.map((f) => {
          const fv = ticket.fields?.find((x) => x.fieldId === f.id);
          return (
            <FieldRow key={f.id} label={f.name} required={f.required}>
              {isEditing
                ? renderFieldEditor(f.id)
                : renderFieldValue(f.id, fv?.value)}
            </FieldRow>
          );
        })}

        <div className="h-px bg-line my-1" />

        <FieldRow label={t("ticketDetailLabelAssignees")}>
          <UserPicker
            multi
            users={slackUsers}
            value={ticket.assigneeIds}
            onChange={(ids) => {
              const before = ticket.assigneeIds;
              const sameLength = before.length === ids.length;
              const sameOrder =
                sameLength && before.every((id, i) => id === ids[i]);
              if (!sameOrder) onChangeAssignees(ids);
            }}
            disabled={isAssigneePending}
            placeholder={t("ticketDetailUnassigned")}
          />
        </FieldRow>
        {ticket.reporterSlackUserId && (
          <FieldRow label={t("ticketDetailLabelReporter")}>
            <SlackUserName
              workspaceId={workspaceId}
              userId={ticket.reporterSlackUserId}
              mute
            />
          </FieldRow>
        )}
        <FieldRow label={t("ticketDetailLabelCreated")}>
          <span className="font-mono text-[12.5px] text-ink-2">
            {new Date(ticket.createdAt).toLocaleString()}
          </span>
        </FieldRow>
        <FieldRow label={t("ticketDetailLabelUpdated")}>
          <span className="font-mono text-[12.5px] text-ink-2">
            {new Date(ticket.updatedAt).toLocaleString()}
          </span>
        </FieldRow>
        {ticket.slackChannelId && (
          <FieldRow label={t("ticketDetailLabelSource")}>
            <span className="inline-flex items-center gap-1 text-[12.5px] text-ink-2">
              <Icon name="slack" size={11} /> #{ticket.slackChannelId}
            </span>
          </FieldRow>
        )}
      </div>
    </Card>
  );
}

function FieldRow({
  label,
  required,
  children,
}: {
  label: string;
  required?: boolean;
  children: ReactNode;
}) {
  return (
    <div className="grid grid-cols-[95px_1fr] gap-2.5 items-center">
      <div className="text-[11.5px] font-medium text-ink-3">
        {label}
        {required && <span className="text-brand ml-0.5">*</span>}
      </div>
      <div className="min-w-0">{children}</div>
    </div>
  );
}

