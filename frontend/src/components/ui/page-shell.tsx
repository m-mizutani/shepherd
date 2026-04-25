import type { ReactNode } from "react";
import { AppHeader } from "./app-header";
import { cn } from "../../lib/utils";

interface Props {
  crumbs?: { label: ReactNode; to?: string }[];
  showSettings?: boolean;
  settingsActive?: boolean;
  rightSlot?: ReactNode;
  children: ReactNode;
  contentClassName?: string;
}

export function PageShell({
  crumbs,
  showSettings,
  settingsActive,
  rightSlot,
  children,
  contentClassName,
}: Props) {
  return (
    <div className="min-h-screen flex flex-col bg-bg text-ink-1">
      <AppHeader
        crumbs={crumbs}
        showSettings={showSettings}
        settingsActive={settingsActive}
        rightSlot={rightSlot}
      />
      <main className={cn("flex-1", contentClassName)}>{children}</main>
    </div>
  );
}
