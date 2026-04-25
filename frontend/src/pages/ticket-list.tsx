import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { useAuth } from "../contexts/auth-context";
import { SlackUserName } from "../components/slack-user-name";

export default function TicketListPage() {
  const { workspaceId } = useParams<{ workspaceId: string }>();
  const { user, logout } = useAuth();

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

  const { data, isLoading, error } = useQuery({
    queryKey: ["tickets", workspaceId],
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/tickets",
        { params: { path: { workspaceId: workspaceId! } } },
      );
      if (error) throw error;
      return data;
    },
  });

  const statusMap = new Map(
    configData?.statuses?.map((s) => [s.id, s]) ?? [],
  );

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200">
        <div className="max-w-5xl mx-auto px-4 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Link to="/" className="text-xl font-bold text-gray-900">
              Shepherd
            </Link>
            <span className="text-gray-400">/</span>
            <span className="text-gray-600">{workspaceId}</span>
          </div>
          <div className="flex items-center gap-4">
            <Link
              to={`/ws/${workspaceId}/settings`}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              Settings
            </Link>
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
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-lg font-semibold text-gray-900">Tickets</h2>
        </div>

        {isLoading && <p className="text-gray-500">Loading...</p>}
        {error && <p className="text-red-600">Failed to load tickets.</p>}

        <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="bg-gray-50 border-b border-gray-200">
                <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                  #
                </th>
                <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                  Title
                </th>
                <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                  Status
                </th>
                <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                  Created
                </th>
              </tr>
            </thead>
            <tbody>
              {data?.tickets?.map((ticket) => {
                const status = statusMap.get(ticket.statusId);
                return (
                  <tr
                    key={ticket.id}
                    className="border-b border-gray-100 hover:bg-gray-50"
                  >
                    <td className="px-4 py-3 text-sm text-gray-500">
                      {ticket.seqNum}
                    </td>
                    <td className="px-4 py-3">
                      <Link
                        to={`/ws/${workspaceId}/tickets/${ticket.id}`}
                        className="text-sm font-medium text-purple-600 hover:text-purple-800"
                      >
                        {ticket.title}
                      </Link>
                    </td>
                    <td className="px-4 py-3">
                      {status && (
                        <span
                          className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
                          style={{
                            backgroundColor: status.color + "20",
                            color: status.color,
                          }}
                        >
                          {status.name}
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-sm text-gray-500">
                      {new Date(ticket.createdAt).toLocaleDateString()}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>

          {data?.tickets?.length === 0 && (
            <div className="px-4 py-8 text-center text-gray-500">
              No tickets yet.
            </div>
          )}
        </div>
      </main>
    </div>
  );
}
