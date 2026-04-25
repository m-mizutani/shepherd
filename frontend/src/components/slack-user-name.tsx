import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";

interface SlackUserNameProps {
  workspaceId: string;
  userId: string;
  showAvatar?: boolean;
}

export function SlackUserName({
  workspaceId,
  userId,
  showAvatar = true,
}: SlackUserNameProps) {
  const { data, isLoading } = useQuery({
    queryKey: ["slack-user", workspaceId, userId],
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/slack/users/{userId}",
        { params: { path: { workspaceId, userId } } },
      );
      if (error) throw error;
      return data;
    },
    staleTime: 3 * 60 * 1000,
    enabled: !!userId,
  });

  if (!userId) return null;

  if (isLoading || !data) {
    return (
      <span className="inline-flex items-center gap-1.5">
        {showAvatar && (
          <span className="w-6 h-6 rounded-full bg-gray-200 animate-pulse shrink-0" />
        )}
        <span className="text-gray-400 text-sm">{userId}</span>
      </span>
    );
  }

  return (
    <span className="inline-flex items-center gap-1.5">
      {showAvatar && data.imageUrl && (
        <img
          src={data.imageUrl}
          alt={data.name}
          className="w-6 h-6 rounded-full shrink-0"
        />
      )}
      {showAvatar && !data.imageUrl && (
        <span className="w-6 h-6 rounded-full bg-gray-300 flex items-center justify-center text-xs text-white font-medium shrink-0">
          {data.name.charAt(0).toUpperCase()}
        </span>
      )}
      <span className="text-sm font-medium text-gray-900">{data.name}</span>
    </span>
  );
}
