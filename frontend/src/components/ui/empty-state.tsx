import type { ReactNode } from "react";
import { Icon, type IconName } from "./icon";

interface Props {
  icon?: IconName;
  title: string;
  description?: ReactNode;
  action?: ReactNode;
}

export function EmptyState({ icon = "folder", title, description, action }: Props) {
  return (
    <div className="flex flex-col items-center gap-2.5 py-10 px-4 text-center text-ink-3">
      <div className="w-14 h-14 rounded-3 bg-brand-soft text-brand flex items-center justify-center">
        <Icon name={icon} size={22} />
      </div>
      <div className="text-[14px] font-semibold text-ink-1">{title}</div>
      {description && (
        <div className="text-[12px] text-ink-3 max-w-md">{description}</div>
      )}
      {action && <div className="mt-1">{action}</div>}
    </div>
  );
}
