import { Link, useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { api } from "../../lib/api";
import { useAuth } from "../../contexts/auth-context";
import { useTheme } from "../../contexts/theme-context";
import { Logo } from "./logo";
import { Icon } from "./icon";
import { Avatar } from "./avatar";
import { Popover, PopoverHeader, PopoverItem, PopoverSep } from "./popover";
import { cn } from "../../lib/utils";

interface Crumb {
  label: ReactNode;
  to?: string;
}

interface Props {
  crumbs?: Crumb[];
  showSettings?: boolean;
  settingsActive?: boolean;
  rightSlot?: ReactNode;
}

export function AppHeader({
  crumbs = [],
  showSettings = false,
  settingsActive = false,
  rightSlot,
}: Props) {
  const { user, logout } = useAuth();
  const { theme, toggle } = useTheme();
  const navigate = useNavigate();
  const { workspaceId } = useParams<{ workspaceId: string }>();

  const { data: wsData } = useQuery({
    queryKey: ["workspaces"],
    queryFn: async () => {
      const { data, error } = await api.GET("/api/v1/ws");
      if (error) throw error;
      return data;
    },
    staleTime: 60_000,
  });

  const workspaces = wsData?.workspaces ?? [];
  const currentWs = workspaces.find((w) => w.id === workspaceId);

  return (
    <header className="h-12 flex items-center px-4 bg-bg-elev border-b border-line shrink-0">
      <Link to="/" className="flex items-center gap-2 font-semibold text-[14px]">
        <Logo />
      </Link>

      {workspaceId && (
        <Popover
          trigger={(toggle, open) => (
            <button
              type="button"
              onClick={toggle}
              className="ml-2 inline-flex items-center gap-1.5 px-2 py-1 rounded-2 text-[13px] text-ink-2 hover:bg-bg-sunken"
            >
              <span className="w-2 h-2 rounded-[2px] bg-brand" />
              <span className="font-medium">{currentWs?.name ?? workspaceId}</span>
              <Icon name={open ? "chevronUp" : "chevron"} size={12} className="text-ink-4" />
            </button>
          )}
        >
          {(close) => (
            <>
              <PopoverHeader>Switch workspace</PopoverHeader>
              {workspaces.map((w) => (
                <PopoverItem
                  key={w.id}
                  active={w.id === workspaceId}
                  onClick={() => {
                    close();
                    navigate(`/ws/${w.id}/tickets`);
                  }}
                >
                  <span className="w-2 h-2 rounded-[2px] bg-brand flex-none" />
                  <span className="flex-1 truncate">{w.name}</span>
                  {w.id === workspaceId && (
                    <Icon name="check" size={12} className="text-brand" />
                  )}
                </PopoverItem>
              ))}
              <PopoverSep />
              <PopoverItem
                onClick={() => {
                  close();
                  navigate("/");
                }}
              >
                <Icon name="folder" size={12} className="text-ink-4" />
                <span>All workspaces</span>
              </PopoverItem>
            </>
          )}
        </Popover>
      )}

      {crumbs.map((c, i) => (
        <span key={i} className="flex items-center">
          <span className="text-ink-5 mx-2 font-light">/</span>
          {c.to ? (
            <Link to={c.to} className="text-ink-2 text-[13px] hover:text-ink-1">
              {c.label}
            </Link>
          ) : (
            <span className="text-ink-2 text-[13px]">{c.label}</span>
          )}
        </span>
      ))}

      <div className="flex-1" />

      {rightSlot}

      <div className="flex items-center gap-1">
        {showSettings && workspaceId && (
          <Link
            to={`/ws/${workspaceId}/settings`}
            className={cn(
              "h-7 px-2.5 inline-flex items-center gap-1.5 rounded-2 text-[12.5px]",
              settingsActive
                ? "bg-bg-sunken text-ink-1"
                : "text-ink-3 hover:bg-bg-sunken hover:text-ink-1",
            )}
          >
            <Icon name="settings" size={13} /> Settings
          </Link>
        )}

        <button
          type="button"
          onClick={toggle}
          title={theme === "dark" ? "Switch to light" : "Switch to dark"}
          className="w-7 h-7 rounded-2 flex items-center justify-center text-ink-3 hover:bg-bg-sunken hover:text-ink-1"
        >
          <Icon name={theme === "dark" ? "sun" : "moon"} size={14} />
        </button>

        <button
          type="button"
          title="Notifications"
          className="relative w-7 h-7 rounded-2 inline-flex items-center justify-center text-ink-3 hover:bg-bg-sunken hover:text-ink-1"
        >
          <Icon name="bell" size={14} />
        </button>
      </div>

      <Popover
        align="end"
        trigger={(toggle) => (
          <button
            type="button"
            onClick={toggle}
            aria-label="User menu"
            className="ml-1.5 flex items-center gap-2 pl-1 pr-1.5 py-1 rounded-2 hover:bg-bg-sunken"
          >
            <Avatar name={user?.name ?? "?"} size="sm" />
            <span className="text-[13px] text-ink-1 font-medium hidden sm:inline">
              {user?.name ?? "User"}
            </span>
            <Icon name="chevron" size={12} className="text-ink-4" />
          </button>
        )}
      >
        {(close) => (
          <>
            <PopoverHeader>Signed in as</PopoverHeader>
            <div className="px-2 py-1 text-[12px] text-ink-3">
              {user?.email ?? user?.name ?? "—"}
            </div>
            <PopoverSep />
            <PopoverItem
              onClick={() => {
                close();
                toggle();
              }}
            >
              <Icon name={theme === "dark" ? "sun" : "moon"} size={13} className="text-ink-4" />
              {theme === "dark" ? "Light theme" : "Dark theme"}
            </PopoverItem>
            <PopoverSep />
            <PopoverItem
              onClick={() => {
                close();
                void logout();
              }}
            >
              <Icon name="arrow" size={13} className="text-ink-4" />
              Sign out
            </PopoverItem>
          </>
        )}
      </Popover>
    </header>
  );
}
