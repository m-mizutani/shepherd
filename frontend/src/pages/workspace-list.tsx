import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { PageShell } from "../components/ui/page-shell";
import { Card } from "../components/ui/card";
import { Icon } from "../components/ui/icon";
import { Skeleton } from "../components/ui/skeleton";
import { ErrorBox } from "../components/ui/error-box";
import { EmptyState } from "../components/ui/empty-state";
import { avatarColorFor } from "../components/ui/avatar";

function initialsOf(name: string): string {
  return (
    name
      .split(/\s|-/)
      .filter(Boolean)
      .slice(0, 2)
      .map((s) => s[0])
      .join("")
      .toUpperCase() || "?"
  );
}

export default function WorkspaceListPage() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["workspaces"],
    queryFn: async () => {
      const { data, error } = await api.GET("/api/v1/ws");
      if (error) throw error;
      return data;
    },
  });

  const workspaces = data?.workspaces ?? [];

  return (
    <PageShell>
      <div className="max-w-[980px] mx-auto px-10 pt-7 pb-10">
        <div className="flex items-baseline justify-between mb-4">
          <div>
            <h1 className="m-0 text-[22px] font-semibold tracking-[-0.018em] text-ink-1">
              Workspaces
            </h1>
            {!isLoading && (
              <div className="text-[12px] text-ink-3 mt-1">
                {workspaces.length > 0
                  ? `Pick a board to triage. You belong to ${workspaces.length}.`
                  : "No workspaces are available to you yet."}
              </div>
            )}
          </div>
        </div>

        {isLoading && (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
            {Array.from({ length: 4 }).map((_, i) => (
              <Card key={i} className="p-4 flex gap-3 items-start">
                <Skeleton width={36} height={36} className="rounded-3 flex-none" />
                <div className="flex-1 space-y-2">
                  <Skeleton width="60%" height={14} />
                  <Skeleton width="40%" height={10} />
                </div>
              </Card>
            ))}
          </div>
        )}

        {error && (
          <ErrorBox
            title="Failed to load workspaces"
            message="Check that the backend is reachable and try again."
            onRetry={() => refetch()}
          />
        )}

        {!isLoading && !error && workspaces.length > 0 && (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3.5">
            {workspaces.map((w) => {
              const color = avatarColorFor(w.id);
              return (
                <Link
                  key={w.id}
                  to={`/ws/${w.id}/tickets`}
                  className="block group"
                >
                  <Card className="p-4 transition-shadow group-hover:shadow-2 group-hover:border-line-strong">
                    <div className="flex items-start gap-3">
                      <div
                        className="w-9 h-9 rounded-3 flex items-center justify-center font-bold text-[13px] flex-none border"
                        style={{
                          background: color + "18",
                          color: color,
                          borderColor: color + "33",
                        }}
                      >
                        {initialsOf(w.name)}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center justify-between">
                          <h3 className="m-0 text-[18px] font-semibold tracking-[-0.012em] text-ink-1 truncate">
                            {w.name}
                          </h3>
                          <Icon name="chevronR" size={14} className="text-ink-4" />
                        </div>
                        <div className="font-mono text-[12px] text-ink-3 mt-0.5">
                          {w.id}
                        </div>
                      </div>
                    </div>
                  </Card>
                </Link>
              );
            })}
          </div>
        )}

        {!isLoading && !error && workspaces.length === 0 && (
          <Card className="p-2">
            <EmptyState
              icon="folder"
              title="No workspaces yet"
              description={
                <>
                  Workspaces are created by admins from the Slack{" "}
                  <code className="font-mono text-brand-ink">
                    /shepherd setup
                  </code>{" "}
                  command. Ask your admin to add you to one.
                </>
              }
            />
          </Card>
        )}

        <div className="mt-6 p-4 border border-dashed border-line-strong rounded-4">
          <div className="flex gap-3.5 items-center">
            <div className="w-14 h-14 rounded-3 bg-brand-soft text-brand flex items-center justify-center">
              <Icon name="folder" size={22} />
            </div>
            <div className="flex-1">
              <div className="text-[14px] font-semibold text-ink-1">
                Looking for another board?
              </div>
              <div className="text-[12px] text-ink-3 mt-0.5">
                Workspaces are created by admins from the Slack{" "}
                <code className="font-mono text-brand-ink">
                  /shepherd setup
                </code>{" "}
                command.
              </div>
            </div>
          </div>
        </div>
      </div>
    </PageShell>
  );
}
