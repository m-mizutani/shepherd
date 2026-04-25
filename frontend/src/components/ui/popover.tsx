import {
  type ReactNode,
  type RefObject,
  useEffect,
  useRef,
  useState,
} from "react";
import { cn } from "../../lib/utils";

export function useClickOutside<T extends HTMLElement>(
  ref: RefObject<T | null>,
  onClose: () => void,
  enabled = true,
) {
  useEffect(() => {
    if (!enabled) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, [ref, onClose, enabled]);
}

interface PopoverProps {
  trigger: (toggle: () => void, isOpen: boolean) => ReactNode;
  children: (close: () => void) => ReactNode;
  align?: "start" | "end";
  className?: string;
}

export function Popover({ trigger, children, align = "start", className }: PopoverProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useClickOutside(ref, () => setOpen(false), open);

  return (
    <div className="relative inline-block" ref={ref}>
      {trigger(() => setOpen((o) => !o), open)}
      {open && (
        <div
          className={cn(
            "absolute top-full mt-1 z-30 bg-bg-elev border border-line rounded-3 shadow-pop p-1 min-w-[180px]",
            align === "end" ? "right-0" : "left-0",
            className,
          )}
        >
          {children(() => setOpen(false))}
        </div>
      )}
    </div>
  );
}

export function PopoverItem({
  children,
  onClick,
  active,
  disabled,
}: {
  children: ReactNode;
  onClick?: () => void;
  active?: boolean;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "w-full flex items-center gap-2 px-2 py-1.5 rounded-2 text-[13px] text-left text-ink-1 cursor-pointer",
        "hover:bg-bg-sunken disabled:opacity-50 disabled:cursor-not-allowed",
        active && "bg-bg-sunken",
      )}
    >
      {children}
    </button>
  );
}

export function PopoverHeader({ children }: { children: ReactNode }) {
  return (
    <div className="text-[11px] text-ink-4 px-2 pt-2 pb-1 uppercase tracking-[0.05em]">
      {children}
    </div>
  );
}

export function PopoverSep() {
  return <div className="h-px bg-line my-1 mx-0.5" />;
}
