import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { useAuth } from "../contexts/auth-context";

export default function WorkspaceListPage() {
  const { user, logout } = useAuth();

  const { data, isLoading, error } = useQuery({
    queryKey: ["workspaces"],
    queryFn: async () => {
      const { data, error } = await api.GET("/api/v1/ws");
      if (error) throw error;
      return data;
    },
  });

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200">
        <div className="max-w-5xl mx-auto px-4 py-4 flex items-center justify-between">
          <h1 className="text-xl font-bold text-gray-900">Shepherd</h1>
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
        <h2 className="text-lg font-semibold text-gray-900 mb-4">
          Workspaces
        </h2>

        {isLoading && <p className="text-gray-500">Loading...</p>}
        {error && (
          <p className="text-red-600">Failed to load workspaces.</p>
        )}

        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {data?.workspaces?.map((ws) => (
            <Link
              key={ws.id}
              to={`/ws/${ws.id}/tickets`}
              className="block p-6 bg-white rounded-lg border border-gray-200 hover:border-purple-300 hover:shadow-sm transition-all"
            >
              <h3 className="font-medium text-gray-900">{ws.name}</h3>
              <p className="text-sm text-gray-500 mt-1">{ws.id}</p>
            </Link>
          ))}
        </div>

        {data?.workspaces?.length === 0 && (
          <p className="text-gray-500">No workspaces configured.</p>
        )}
      </main>
    </div>
  );
}
