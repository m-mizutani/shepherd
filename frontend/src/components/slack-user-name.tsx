import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import { Avatar, type AvatarSize } from "./ui/avatar";
import { cn } from "../lib/utils";

interface SlackUserNameProps {
  workspaceId: string;
  userId: string;
  showAvatar?: boolean;
  size?: AvatarSize;
  mute?: boolean;
  className?: string;
}

export function SlackUserName({
  workspaceId,
  userId,
  showAvatar = true,
  size = "sm",
  mute,
  className,
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
      <span className={cn("inline-flex items-center gap-1.5", className)}>
        {showAvatar && (
          <span className="w-5 h-5 rounded-1 bg-bg-sunken animate-shp-pulse shrink-0" />
        )}
        <span className="text-ink-4 text-[12.5px]">{userId}</span>
      </span>
    );
  }

  return (
    <span className={cn("inline-flex items-center gap-1.5", className)}>
      {showAvatar && (
        <Avatar name={data.name} src={data.imageUrl} size={size} />
      )}
      <span
        className={cn(
          "text-[13px] whitespace-nowrap",
          mute ? "text-ink-3 font-normal" : "text-ink-1 font-medium",
        )}
      >
        {data.name}
      </span>
    </span>
  );
}
