import { type ButtonHTMLAttributes, forwardRef } from "react";
import { cn } from "../../lib/utils";

type Variant = "default" | "primary" | "ghost" | "slack" | "danger";
type Size = "sm" | "md" | "lg";

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  size?: Size;
}

const VARIANT: Record<Variant, string> = {
  default:
    "bg-bg-elev text-ink-1 border-line-strong shadow-1 hover:bg-bg-sunken",
  primary:
    "bg-brand text-white border-brand-ink shadow-[0_1px_0_rgba(0,0,0,.08),inset_0_1px_0_rgba(255,255,255,.18)] hover:bg-brand-ink",
  ghost:
    "bg-transparent border-transparent text-ink-2 hover:bg-bg-sunken hover:text-ink-1",
  slack:
    "bg-white text-ink-1 border-line-strong shadow-1 hover:bg-[#faf8f3]",
  danger:
    "bg-bg-elev text-danger border-line-strong hover:bg-danger-soft",
};

const SIZE: Record<Size, string> = {
  sm: "h-[26px] px-2.5 text-[12px] gap-1",
  md: "h-[30px] px-3 text-[13px] gap-1.5",
  lg: "h-[38px] px-4 text-[14px] gap-2",
};

export const Button = forwardRef<HTMLButtonElement, Props>(function Button(
  { variant = "default", size = "md", className, children, ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      className={cn(
        "inline-flex items-center justify-center font-medium rounded-2 border cursor-pointer transition-colors disabled:opacity-50 disabled:cursor-not-allowed disabled:pointer-events-none",
        VARIANT[variant],
        SIZE[size],
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  );
});
