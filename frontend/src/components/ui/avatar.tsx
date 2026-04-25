import { cn } from "../../lib/utils";

const PALETTE: Record<string, string> = {
  purple: "#7c3aed",
  orange: "#c2410c",
  green: "#15803d",
  blue: "#1d4ed8",
  pink: "#be185d",
  teal: "#0e7490",
  red: "#b91c1c",
  amber: "#a16207",
};
const PALETTE_KEYS = Object.keys(PALETTE);

export function avatarColorFor(seed: string): string {
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) >>> 0;
  return PALETTE[PALETTE_KEYS[h % PALETTE_KEYS.length]];
}

export type AvatarSize = "xs" | "sm" | "md" | "lg";

const SIZE_CLASS: Record<AvatarSize, string> = {
  xs: "w-4 h-4 text-[9px] rounded-[3px]",
  sm: "w-5 h-5 text-[10px] rounded-[4px]",
  md: "w-6 h-6 text-[11px] rounded-[4px]",
  lg: "w-8 h-8 text-[12px] rounded-[6px]",
};

interface AvatarProps {
  name: string;
  src?: string;
  size?: AvatarSize;
  color?: string;
  className?: string;
}

export function Avatar({
  name,
  src,
  size = "sm",
  color,
  className,
}: AvatarProps) {
  const initials =
    name
      .split(/\s|@|-/)
      .filter(Boolean)
      .slice(0, 2)
      .map((s) => s[0])
      .join("")
      .toUpperCase() || "?";
  const c = color || avatarColorFor(name);
  return (
    <span
      className={cn(
        "inline-flex items-center justify-center font-semibold flex-none overflow-hidden",
        SIZE_CLASS[size],
        className,
      )}
      style={
        src ? { background: "var(--bg-sunken)" } : { background: `${c}22`, color: c }
      }
    >
      {src ? (
        <img src={src} alt={name} className="w-full h-full object-cover" />
      ) : (
        initials
      )}
    </span>
  );
}

interface UserChipProps {
  name: string;
  src?: string;
  size?: AvatarSize;
  color?: string;
  mute?: boolean;
  className?: string;
}

export function UserChip({
  name,
  src,
  size = "sm",
  color,
  mute,
  className,
}: UserChipProps) {
  return (
    <span className={cn("inline-flex items-center gap-1.5", className)}>
      <Avatar name={name} src={src} size={size} color={color} />
      <span
        className={cn(
          "text-[13px] whitespace-nowrap",
          mute ? "text-ink-3 font-normal" : "text-ink-1 font-medium",
        )}
      >
        {name}
      </span>
    </span>
  );
}
