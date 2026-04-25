import { useAuth } from "../contexts/auth-context";
import { Navigate } from "react-router-dom";

export default function LoginPage() {
  const { user, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <p className="text-gray-500">Loading...</p>
      </div>
    );
  }

  if (user) {
    return <Navigate to="/" replace />;
  }

  return (
    <div className="flex items-center justify-center min-h-screen bg-gray-50">
      <div className="w-full max-w-sm p-8 bg-white rounded-lg shadow-md">
        <h1 className="mb-6 text-2xl font-bold text-center text-gray-900">
          Shepherd
        </h1>
        <p className="mb-6 text-center text-gray-600">
          Sign in with your Slack account to continue.
        </p>
        <a
          href="/api/auth/login"
          className="flex items-center justify-center w-full px-4 py-2 text-white bg-purple-600 rounded-md hover:bg-purple-700 transition-colors"
        >
          Sign in with Slack
        </a>
      </div>
    </div>
  );
}
