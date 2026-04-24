import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { useAuth } from "../contexts/auth-context";

export default function TicketDetailPage() {
  const { workspaceId, ticketId } = useParams<{
    workspaceId: string;
    ticketId: string;
  }>();
  const { user, logout } = useAuth();
  const queryClient = useQueryClient();

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

  const updateStatus = useMutation({
    mutationFn: async (statusId: string) => {
      const { data, error } = await api.PATCH(
        "/api/v1/ws/{workspaceId}/tickets/{ticketId}",
        {
          params: {
            path: { workspaceId: workspaceId!, ticketId: ticketId! },
          },
          body: { statusId },
        },
      );
      if (error) throw error;
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["ticket", workspaceId, ticketId],
      });
    },
  });

  const statusMap = new Map(
    configData?.statuses?.map((s) => [s.id, s]) ?? [],
  );
  const currentStatus = ticket ? statusMap.get(ticket.statusId) : null;

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
            <span className="text-sm text-gray-600">{user?.name}</span>
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
            <div className="col-span-2 space-y-6">
              <div>
                <div className="flex items-center gap-2 text-sm text-gray-500 mb-2">
                  <span>#{ticket.seqNum}</span>
                </div>
                <h2 className="text-2xl font-bold text-gray-900">
                  {ticket.title}
                </h2>
              </div>

              {ticket.description && (
                <div className="bg-white p-4 rounded-lg border border-gray-200">
                  <p className="text-gray-700 whitespace-pre-wrap">
                    {ticket.description}
                  </p>
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
                        <span className="text-sm font-medium text-gray-900">
                          {comment.slackUserId}
                        </span>
                        <span className="text-xs text-gray-500">
                          {new Date(comment.createdAt).toLocaleString()}
                        </span>
                      </div>
                      <p className="text-sm text-gray-700 whitespace-pre-wrap">
                        {comment.body}
                      </p>
                    </div>
                  ))}
                  {commentsData?.comments?.length === 0 && (
                    <p className="text-sm text-gray-500">No comments yet.</p>
                  )}
                </div>
              </div>
            </div>

            <div className="space-y-4">
              <div className="bg-white p-4 rounded-lg border border-gray-200">
                <h3 className="text-sm font-medium text-gray-900 mb-3">
                  Status
                </h3>
                <div className="space-y-1">
                  {configData?.statuses?.map((status) => (
                    <button
                      key={status.id}
                      onClick={() => updateStatus.mutate(status.id)}
                      disabled={status.id === ticket.statusId}
                      className={`w-full text-left px-3 py-1.5 rounded text-sm transition-colors ${
                        status.id === ticket.statusId
                          ? "font-medium"
                          : "hover:bg-gray-50 text-gray-600"
                      }`}
                      style={
                        status.id === ticket.statusId
                          ? {
                              backgroundColor: status.color + "20",
                              color: status.color,
                            }
                          : undefined
                      }
                    >
                      {status.name}
                    </button>
                  ))}
                </div>
              </div>

              {ticket.fields && ticket.fields.length > 0 && (
                <div className="bg-white p-4 rounded-lg border border-gray-200">
                  <h3 className="text-sm font-medium text-gray-900 mb-3">
                    Fields
                  </h3>
                  <div className="space-y-2">
                    {ticket.fields.map((field) => {
                      const fieldDef = configData?.fields?.find(
                        (f) => f.id === field.fieldId,
                      );
                      return (
                        <div key={field.fieldId}>
                          <dt className="text-xs text-gray-500">
                            {fieldDef?.name ?? field.fieldId}
                          </dt>
                          <dd className="text-sm text-gray-900">
                            {String(field.value)}
                          </dd>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}

              {currentStatus && (
                <div className="bg-white p-4 rounded-lg border border-gray-200">
                  <h3 className="text-sm font-medium text-gray-900 mb-2">
                    Info
                  </h3>
                  <dl className="space-y-2 text-sm">
                    {ticket.assigneeId && (
                      <div>
                        <dt className="text-gray-500">Assignee</dt>
                        <dd className="text-gray-900">{ticket.assigneeId}</dd>
                      </div>
                    )}
                    {ticket.reporterSlackUserId && (
                      <div>
                        <dt className="text-gray-500">Reporter</dt>
                        <dd className="text-gray-900">
                          {ticket.reporterSlackUserId}
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
              )}
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
