import { type ReactNode, useEffect } from "react";
import { cn } from "../../lib/utils";

interface Props {
  open: boolean;
  onClose: () => void;
  title?: string;
  children: ReactNode;
  footer?: ReactNode;
  width?: number;
}

export function Dialog({ open, onClose, title, children, footer, width = 480 }: Props) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-40 flex items-start justify-center pt-24 px-4 bg-black/30 backdrop-blur-[2px]">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={cn(
          "bg-bg-elev border border-line rounded-4 shadow-pop w-full max-w-full",
        )}
        style={{ width }}
        onClick={(e) => e.stopPropagation()}
      >
        {title && (
          <div className="px-4 py-3 border-b border-line flex items-center justify-between">
            <h2 className="text-[14px] font-semibold text-ink-1 m-0">{title}</h2>
            <button
              type="button"
              onClick={onClose}
              className="text-ink-4 hover:text-ink-1 w-6 h-6 inline-flex items-center justify-center rounded-1 hover:bg-bg-sunken"
              aria-label="Close"
            >
              ×
            </button>
          </div>
        )}
        <div className="p-4">{children}</div>
        {footer && (
          <div className="px-4 py-3 border-t border-line flex items-center justify-end gap-2">
            {footer}
          </div>
        )}
      </div>
      <div className="fixed inset-0 -z-10" onClick={onClose} aria-hidden />
    </div>
  );
}
