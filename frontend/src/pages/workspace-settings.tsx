import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { useAuth } from "../contexts/auth-context";

export default function WorkspaceSettingsPage() {
  const { workspaceId } = useParams<{ workspaceId: string }>();
  const { user, logout } = useAuth();

  const { data, isLoading, error } = useQuery({
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
            <span className="text-gray-400">/</span>
            <span className="text-gray-600">Settings</span>
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

      <main className="max-w-5xl mx-auto px-4 py-8 space-y-8">
        {isLoading && <p className="text-gray-500">Loading...</p>}
        {error && <p className="text-red-600">Failed to load config.</p>}

        {data && (
          <>
            <section>
              <h2 className="text-lg font-semibold text-gray-900 mb-4">
                Statuses
              </h2>
              <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
                <table className="w-full">
                  <thead>
                    <tr className="bg-gray-50 border-b border-gray-200">
                      <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                        Name
                      </th>
                      <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                        ID
                      </th>
                      <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                        Color
                      </th>
                      <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                        Closed
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.statuses.map((status) => (
                      <tr
                        key={status.id}
                        className="border-b border-gray-100"
                      >
                        <td className="px-4 py-3">
                          <span
                            className="inline-flex items-center px-2 py-0.5 rounded text-sm font-medium"
                            style={{
                              backgroundColor: status.color + "20",
                              color: status.color,
                            }}
                          >
                            {status.name}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-sm text-gray-500">
                          {status.id}
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-2">
                            <div
                              className="w-4 h-4 rounded"
                              style={{ backgroundColor: status.color }}
                            />
                            <span className="text-sm text-gray-500">
                              {status.color}
                            </span>
                          </div>
                        </td>
                        <td className="px-4 py-3 text-sm text-gray-500">
                          {status.isClosed ? "Yes" : "No"}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>

            <section>
              <h2 className="text-lg font-semibold text-gray-900 mb-4">
                Fields
              </h2>
              <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
                <table className="w-full">
                  <thead>
                    <tr className="bg-gray-50 border-b border-gray-200">
                      <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                        Name
                      </th>
                      <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                        ID
                      </th>
                      <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                        Type
                      </th>
                      <th className="text-left px-4 py-3 text-sm font-medium text-gray-500">
                        Required
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.fields.map((field) => (
                      <tr
                        key={field.id}
                        className="border-b border-gray-100"
                      >
                        <td className="px-4 py-3 text-sm font-medium text-gray-900">
                          {field.name}
                        </td>
                        <td className="px-4 py-3 text-sm text-gray-500">
                          {field.id}
                        </td>
                        <td className="px-4 py-3 text-sm text-gray-500">
                          {field.type}
                        </td>
                        <td className="px-4 py-3 text-sm text-gray-500">
                          {field.required ? "Yes" : "No"}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>

            <section>
              <h2 className="text-lg font-semibold text-gray-900 mb-4">
                Ticket Config
              </h2>
              <div className="bg-white p-4 rounded-lg border border-gray-200">
                <dl className="space-y-2 text-sm">
                  <div>
                    <dt className="text-gray-500">Default Status</dt>
                    <dd className="text-gray-900">
                      {data.ticketConfig.defaultStatusId}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-gray-500">Closed Statuses</dt>
                    <dd className="text-gray-900">
                      {data.ticketConfig.closedStatusIds.join(", ") || "None"}
                    </dd>
                  </div>
                </dl>
              </div>
            </section>

            <section>
              <h2 className="text-lg font-semibold text-gray-900 mb-4">
                Labels
              </h2>
              <div className="bg-white p-4 rounded-lg border border-gray-200">
                <dl className="space-y-2 text-sm">
                  <div>
                    <dt className="text-gray-500">Ticket</dt>
                    <dd className="text-gray-900">{data.labels.ticket}</dd>
                  </div>
                  <div>
                    <dt className="text-gray-500">Title</dt>
                    <dd className="text-gray-900">{data.labels.title}</dd>
                  </div>
                  <div>
                    <dt className="text-gray-500">Description</dt>
                    <dd className="text-gray-900">
                      {data.labels.description}
                    </dd>
                  </div>
                </dl>
              </div>
            </section>
          </>
        )}
      </main>
    </div>
  );
}
