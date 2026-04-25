import { useState, useRef, useEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { useAuth } from "../contexts/auth-context";
import { SlackUserName } from "../components/slack-user-name";
import { SlackMarkdown } from "../components/slack-markdown";
import { UserPicker } from "../components/user-picker";

function isValidURL(s: string): boolean {
  try {
    new URL(s);
    return true;
  } catch {
    return false;
  }
}

export default function TicketDetailPage() {
  const { workspaceId, ticketId } = useParams<{
    workspaceId: string;
    ticketId: string;
  }>();
  const { user, logout } = useAuth();
  const queryClient = useQueryClient();

  const [isEditing, setIsEditing] = useState(false);
  const [editTitle, setEditTitle] = useState("");
  const [editDescription, setEditDescription] = useState("");
  const [editAssigneeId, setEditAssigneeId] = useState("");
  const [editFields, setEditFields] = useState<
    Record<string, unknown>
  >({});
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [statusDropdownOpen, setStatusDropdownOpen] = useState(false);
  const statusRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (statusRef.current && !statusRef.current.contains(e.target as Node)) {
        setStatusDropdownOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

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
      assigneeId?: string;
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
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["ticket", workspaceId, ticketId],
      });
      setIsEditing(false);
    },
  });

  const statusMap = new Map(
    configData?.statuses?.map((s) => [s.id, s]) ?? [],
  );
  const currentStatus = ticket ? statusMap.get(ticket.statusId) : null;
  const fieldMap = new Map(
    configData?.fields?.map((f) => [f.id, f]) ?? [],
  );

  const startEditing = () => {
    if (!ticket) return;
    setEditTitle(ticket.title);
    setEditDescription(ticket.description ?? "");
    setEditAssigneeId(ticket.assigneeId ?? "");
    const fields: Record<string, unknown> = {};
    for (const f of ticket.fields ?? []) {
      fields[f.fieldId] = f.value;
    }
    setEditFields(fields);
    setIsEditing(true);
  };

  const saveEdits = () => {
    const errors: Record<string, string> = {};
    for (const [fieldId, value] of Object.entries(editFields)) {
      const def = fieldMap.get(fieldId);
      if (def?.type === "url" && value && !isValidURL(String(value))) {
        errors[fieldId] = "Invalid URL";
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
      assigneeId: editAssigneeId,
      fields: fieldValues,
    });
  };

  const renderFieldValue = (
    fieldId: string,
    value: unknown,
  ) => {
    const def = fieldMap.get(fieldId);
    if (!def) return String(value ?? "");

    if (value === null || value === undefined || value === "") {
      return <span className="text-gray-400">—</span>;
    }

    switch (def.type) {
      case "url":
        return (
          <a
            href={String(value)}
            target="_blank"
            rel="noopener noreferrer"
            className="text-blue-600 hover:underline text-sm break-all"
          >
            {String(value)}
          </a>
        );
      case "user":
        return (
          <SlackUserName
            workspaceId={workspaceId!}
            userId={String(value)}
          />
        );
      case "multi-user": {
        const ids = Array.isArray(value) ? value : [value];
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
        if (opt) {
          return (
            <span
              className="inline-block px-2 py-0.5 rounded-full text-xs font-medium"
              style={
                opt.color
                  ? { backgroundColor: opt.color + "20", color: opt.color }
                  : { backgroundColor: "#e5e7eb", color: "#374151" }
              }
            >
              {opt.name}
            </span>
          );
        }
        return <span className="text-sm">{String(value)}</span>;
      }
      case "multi-select": {
        const values = Array.isArray(value) ? value : [value];
        return (
          <div className="flex flex-wrap gap-1">
            {values.map((v) => {
              const opt = def.options?.find((o) => o.id === String(v));
              return (
                <span
                  key={String(v)}
                  className="inline-block px-2 py-0.5 rounded-full text-xs font-medium"
                  style={
                    opt?.color
                      ? { backgroundColor: opt.color + "20", color: opt.color }
                      : { backgroundColor: "#e5e7eb", color: "#374151" }
                  }
                >
                  {opt?.name ?? String(v)}
                </span>
              );
            })}
          </div>
        );
      }
      case "date":
        return (
          <span className="text-sm">
            {new Date(String(value)).toLocaleDateString()}
          </span>
        );
      default:
        return <span className="text-sm">{String(value)}</span>;
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
            className="w-full border border-gray-300 rounded px-2 py-1 text-sm"
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
            className="w-full border border-gray-300 rounded px-2 py-1 text-sm"
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
                [fieldId]: e.target.valueAsNumber || "",
              })
            }
            className="w-full border border-gray-300 rounded px-2 py-1 text-sm"
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
            className="w-full border border-gray-300 rounded px-2 py-1 text-sm"
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
                  setFieldErrors({ ...fieldErrors, [fieldId]: "Invalid URL" });
                } else {
                  const { [fieldId]: _, ...rest } = fieldErrors;
                  setFieldErrors(rest);
                }
              }}
              placeholder="https://example.com"
              className={`w-full border rounded px-2 py-1 text-sm ${fieldErrors[fieldId] ? "border-red-300" : "border-gray-300"}`}
            />
            {fieldErrors[fieldId] && (
              <p className="text-xs text-red-500 mt-0.5">{fieldErrors[fieldId]}</p>
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
            placeholder="Select users..."
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
            className="w-full border border-gray-300 rounded px-2 py-1 text-sm"
          />
        );
    }
  };

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200">
        <div className="max-w-5xl mx-auto px-4 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Link to="/" className="text-xl font-bold text-gray-900">
              Shepherd
            </Link>
            <span className="text-gray-400">/</span>
            <Link
              to={`/ws/${workspaceId}/tickets`}
              className="text-gray-600 hover:text-gray-900"
            >
              {workspaceId}
            </Link>
          </div>
          <div className="flex items-center gap-4">
            {user?.sub ? (
              <SlackUserName workspaceId={workspaceId!} userId={user.sub} />
            ) : (
              <span className="text-sm text-gray-600">{user?.name}</span>
            )}
            <button
              onClick={logout}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-5xl mx-auto px-4 py-8">
        {isLoading && <p className="text-gray-500">Loading...</p>}
        {error && <p className="text-red-600">Failed to load ticket.</p>}

        {ticket && (
          <div className="grid grid-cols-3 gap-8">
            {/* Left: Main content */}
            <div className="col-span-2 space-y-6">
              <div>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm text-gray-500">
                    #{ticket.seqNum}
                  </span>
                  {!isEditing && (
                    <button
                      onClick={startEditing}
                      className="px-3 py-1 text-sm text-gray-600 border border-gray-300 rounded-md hover:bg-gray-50 transition-colors"
                    >
                      Edit
                    </button>
                  )}
                </div>

                {isEditing ? (
                  <input
                    type="text"
                    value={editTitle}
                    onChange={(e) => setEditTitle(e.target.value)}
                    className="w-full text-2xl font-bold text-gray-900 border border-gray-300 rounded-md px-3 py-1"
                  />
                ) : (
                  <h2 className="text-2xl font-bold text-gray-900">
                    {ticket.title}
                  </h2>
                )}
              </div>

              {isEditing ? (
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Description
                  </label>
                  <textarea
                    value={editDescription}
                    onChange={(e) => setEditDescription(e.target.value)}
                    rows={6}
                    className="w-full border border-gray-300 rounded-md px-3 py-2 text-sm"
                  />
                </div>
              ) : (
                ticket.description && (
                  <div className="bg-white p-4 rounded-lg border border-gray-200">
                    <SlackMarkdown
                      text={ticket.description}
                      className="text-gray-700 whitespace-pre-wrap"
                    />
                  </div>
                )
              )}

              {isEditing && (
                <div className="flex gap-2">
                  <button
                    onClick={saveEdits}
                    disabled={updateTicket.isPending}
                    className="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-md hover:bg-blue-700 disabled:opacity-50"
                  >
                    {updateTicket.isPending ? "Saving..." : "Save"}
                  </button>
                  <button
                    onClick={() => setIsEditing(false)}
                    className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
                  >
                    Cancel
                  </button>
                  {updateTicket.isError && (
                    <span className="text-sm text-red-600 self-center">
                      Failed to save changes.
                    </span>
                  )}
                </div>
              )}

              <div>
                <h3 className="text-sm font-medium text-gray-900 mb-3">
                  Comments
                </h3>
                <div className="space-y-3">
                  {commentsData?.comments?.map((comment) => (
                    <div
                      key={comment.id}
                      className="bg-white p-4 rounded-lg border border-gray-200"
                    >
                      <div className="flex items-center gap-2 mb-2">
                        <SlackUserName
                          workspaceId={workspaceId!}
                          userId={comment.slackUserId}
                        />
                        <span className="text-xs text-gray-500">
                          {new Date(comment.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <SlackMarkdown
                        text={comment.body}
                        className="text-sm text-gray-700 whitespace-pre-wrap"
                      />
                    </div>
                  ))}
                  {commentsData?.comments?.length === 0 && (
                    <p className="text-sm text-gray-500">No comments yet.</p>
                  )}
                </div>
              </div>
            </div>

            {/* Right: Sidebar */}
            <div className="space-y-4">
              {/* Status */}
              <div className="bg-white p-4 rounded-lg border border-gray-200">
                <h3 className="text-sm font-medium text-gray-900 mb-3">
                  Status
                </h3>
                <div className="relative" ref={statusRef}>
                  <button
                    onClick={() => setStatusDropdownOpen(!statusDropdownOpen)}
                    className="w-full flex items-center justify-between px-3 py-2 rounded-md border border-gray-200 text-sm font-medium transition-colors hover:border-gray-300"
                    style={
                      currentStatus
                        ? {
                            backgroundColor: currentStatus.color + "15",
                            color: currentStatus.color,
                            borderColor: currentStatus.color + "40",
                          }
                        : undefined
                    }
                  >
                    <div className="flex items-center gap-2">
                      {currentStatus && (
                        <span
                          className="w-2.5 h-2.5 rounded-full"
                          style={{ backgroundColor: currentStatus.color }}
                        />
                      )}
                      <span>{currentStatus?.name ?? ticket.statusId}</span>
                    </div>
                    <svg
                      className={`w-4 h-4 transition-transform ${statusDropdownOpen ? "rotate-180" : ""}`}
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M19 9l-7 7-7-7"
                      />
                    </svg>
                  </button>

                  {statusDropdownOpen && (
                    <div className="absolute z-10 mt-1 w-full bg-white border border-gray-200 rounded-md shadow-lg">
                      {configData?.statuses?.map((status) => (
                        <button
                          key={status.id}
                          onClick={() => {
                            if (status.id !== ticket.statusId) {
                              updateTicket.mutate({ statusId: status.id });
                            }
                            setStatusDropdownOpen(false);
                          }}
                          className={`w-full flex items-center gap-2 px-3 py-2 text-sm text-left transition-colors hover:bg-gray-50 first:rounded-t-md last:rounded-b-md ${
                            status.id === ticket.statusId
                              ? "font-medium"
                              : "text-gray-700"
                          }`}
                        >
                          <span
                            className="w-2.5 h-2.5 rounded-full shrink-0"
                            style={{ backgroundColor: status.color }}
                          />
                          <span>{status.name}</span>
                          {status.id === ticket.statusId && (
                            <svg
                              className="w-4 h-4 ml-auto text-gray-500"
                              fill="none"
                              stroke="currentColor"
                              viewBox="0 0 24 24"
                            >
                              <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth={2}
                                d="M5 13l4 4L19 7"
                              />
                            </svg>
                          )}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </div>

              {/* Fields */}
              {configData?.fields && configData.fields.length > 0 && (
                <div className="bg-white p-4 rounded-lg border border-gray-200">
                  <h3 className="text-sm font-medium text-gray-900 mb-3">
                    Fields
                  </h3>
                  <dl className="space-y-3">
                    {configData.fields.map((fieldDef) => {
                      const fieldValue = ticket.fields?.find(
                        (f) => f.fieldId === fieldDef.id,
                      );
                      return (
                        <div key={fieldDef.id}>
                          <dt className="text-xs text-gray-500 mb-0.5">
                            {fieldDef.name}
                            {fieldDef.required && (
                              <span className="text-red-400 ml-0.5">*</span>
                            )}
                          </dt>
                          <dd>
                            {isEditing
                              ? renderFieldEditor(fieldDef.id)
                              : renderFieldValue(
                                  fieldDef.id,
                                  fieldValue?.value,
                                )}
                          </dd>
                        </div>
                      );
                    })}
                  </dl>
                </div>
              )}

              {/* Info */}
              <div className="bg-white p-4 rounded-lg border border-gray-200">
                <h3 className="text-sm font-medium text-gray-900 mb-2">
                  Info
                </h3>
                <dl className="space-y-3 text-sm">
                  <div>
                    <dt className="text-gray-500 mb-0.5">Assignee</dt>
                    <dd>
                      {isEditing ? (
                        <UserPicker
                          users={slackUsersData?.users ?? []}
                          value={editAssigneeId}
                          onChange={setEditAssigneeId}
                        />
                      ) : ticket.assigneeId ? (
                        <SlackUserName
                          workspaceId={workspaceId!}
                          userId={ticket.assigneeId}
                        />
                      ) : (
                        <span className="text-gray-400">—</span>
                      )}
                    </dd>
                  </div>
                  {ticket.reporterSlackUserId && (
                    <div>
                      <dt className="text-gray-500 mb-0.5">Reporter</dt>
                      <dd>
                        <SlackUserName
                          workspaceId={workspaceId!}
                          userId={ticket.reporterSlackUserId}
                        />
                      </dd>
                    </div>
                  )}
                  <div>
                    <dt className="text-gray-500">Created</dt>
                    <dd className="text-gray-900">
                      {new Date(ticket.createdAt).toLocaleString()}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-gray-500">Updated</dt>
                    <dd className="text-gray-900">
                      {new Date(ticket.updatedAt).toLocaleString()}
                    </dd>
                  </div>
                </dl>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
