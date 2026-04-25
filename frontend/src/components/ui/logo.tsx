import logoSrc from "../../assets/logo.png";
import { cn } from "../../lib/utils";

interface Props {
  size?: number;
  withWord?: boolean;
  className?: string;
}

export function Logo({ size = 22, withWord = true, className }: Props) {
  return (
    <span className={cn("inline-flex items-center gap-1.5", className)}>
      <span
        className="rounded-2 border border-[#f1d6b6] bg-brand-soft flex items-center justify-center overflow-hidden flex-none"
        style={{ width: size, height: size }}
      >
        <img
          src={logoSrc}
          alt=""
          className="object-contain"
          style={{ width: size + 2, height: size + 2, marginTop: 1 }}
        />
      </span>
      {withWord && (
        <span className="font-semibold text-[14px] tracking-[-0.01em] text-ink-1">
          Shepherd
        </span>
      )}
    </span>
  );
}

export { logoSrc };
