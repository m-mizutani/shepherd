import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "../../lib/utils";

export function Card({
  className,
  children,
  ...rest
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "bg-bg-elev border border-line rounded-4 shadow-1",
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  );
}

interface CardHeaderProps {
  title: ReactNode;
  action?: ReactNode;
  className?: string;
}

export function CardHeader({ title, action, className }: CardHeaderProps) {
  return (
    <div
      className={cn(
        "px-3.5 pt-3 flex items-center justify-between gap-2",
        className,
      )}
    >
      <h3 className="m-0 text-[12px] font-semibold uppercase tracking-[0.05em] text-ink-3">
        {title}
      </h3>
      {action}
    </div>
  );
}

export function CardBody({
  className,
  children,
  ...rest
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("px-3.5 pb-3.5 pt-3", className)} {...rest}>
      {children}
    </div>
  );
}
