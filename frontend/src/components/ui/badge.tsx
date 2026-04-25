import type { CSSProperties, ReactNode } from "react";
import { cn } from "../../lib/utils";

export type BadgeTone =
  | "neutral"
  | "info"
  | "success"
  | "warn"
  | "danger"
  | "brand";

const TONE_COLORS: Record<BadgeTone, { bg: string; fg: string }> = {
  neutral: { bg: "#e5e7eb", fg: "#4b5563" },
  info: { bg: "#dbeafe", fg: "#1d4ed8" },
  success: { bg: "#dcfce7", fg: "#15803d" },
  warn: { bg: "#fef3c7", fg: "#a16207" },
  danger: { bg: "#fee2e2", fg: "#b91c1c" },
  brand: { bg: "#ffe4ce", fg: "#c2410c" },
};

interface Props {
  children: ReactNode;
  tone?: BadgeTone;
  /** Custom hex color; bg auto-derived as color + 22 alpha. */
  color?: string;
  size?: "sm" | "md";
  dot?: boolean;
  className?: string;
  style?: CSSProperties;
}

export function Badge({
  children,
  tone,
  color,
  size = "sm",
  dot = true,
  className,
  style,
}: Props) {
  const palette = color
    ? { bg: `${color}22`, fg: color }
    : TONE_COLORS[tone ?? "neutral"];

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full leading-none font-medium",
        size === "md"
          ? "h-[26px] px-2.5 text-[12.5px]"
          : "h-[22px] px-2 text-[11.5px]",
        className,
      )}
      style={{ background: palette.bg, color: palette.fg, ...style }}
    >
      {dot && (
        <span
          className="w-1.5 h-1.5 rounded-full"
          style={{ background: palette.fg }}
        />
      )}
      {children}
    </span>
  );
}
