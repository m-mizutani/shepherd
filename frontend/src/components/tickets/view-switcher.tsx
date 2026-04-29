import { Icon, type IconName } from "../ui/icon";
import { useTranslation } from "../../i18n";
import type { MsgKey } from "../../i18n/keys";
import { cn } from "../../lib/utils";

export type TicketView = "list" | "board";

const ITEMS: ReadonlyArray<{
  id: TicketView;
  icon: IconName;
  labelKey: MsgKey;
}> = [
  { id: "list", icon: "sort", labelKey: "ticketViewList" },
  { id: "board", icon: "grip", labelKey: "ticketViewBoard" },
];

export function ViewSwitcher({
  current,
  onChange,
}: {
  current: TicketView;
  onChange: (v: TicketView) => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="inline-flex border border-line-strong rounded-2 overflow-hidden bg-bg-elev">
      {ITEMS.map((it, i) => {
        const active = it.id === current;
        return (
          <button
            key={it.id}
            type="button"
            onClick={() => !active && onChange(it.id)}
            className={cn(
              "h-7 px-2.5 inline-flex items-center gap-1.5 text-[12px] font-medium",
              i < ITEMS.length - 1 && "border-r border-line",
              active
                ? "bg-bg-sunken text-ink-1"
                : "text-ink-3 hover:bg-bg-sunken",
            )}
            aria-pressed={active}
          >
            <Icon name={it.icon} size={12} /> {t(it.labelKey)}
          </button>
        );
      })}
    </div>
  );
}
