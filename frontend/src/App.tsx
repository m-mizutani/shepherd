import { Routes, Route, Navigate } from "react-router-dom";
import { useAuth } from "./contexts/auth-context";
import LoginPage from "./pages/login";
import WorkspaceListPage from "./pages/workspace-list";
import TicketListPage from "./pages/ticket-list";
import TicketDetailPage from "./pages/ticket-detail";
import WorkspaceSettingsPage from "./pages/workspace-settings";

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <p className="text-gray-500">Loading...</p>
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <WorkspaceListPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/ws/:workspaceId/tickets"
        element={
          <ProtectedRoute>
            <TicketListPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/ws/:workspaceId/tickets/:ticketId"
        element={
          <ProtectedRoute>
            <TicketDetailPage />
          </ProtectedRoute>
        }
      />
      <Route
        path="/ws/:workspaceId/settings"
        element={
          <ProtectedRoute>
            <WorkspaceSettingsPage />
          </ProtectedRoute>
        }
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
