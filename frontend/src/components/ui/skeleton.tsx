import { cn } from "../../lib/utils";

interface Props {
  className?: string;
  width?: number | string;
  height?: number | string;
}

export function Skeleton({ className, width, height }: Props) {
  return (
    <span
      className={cn(
        "block bg-bg-sunken rounded-1 animate-shp-pulse",
        className,
      )}
      style={{ width, height: height ?? 12 }}
    />
  );
}

export function SkeletonRow({ widths }: { widths: (number | string)[] }) {
  return (
    <div className="flex items-center gap-3 py-2">
      {widths.map((w, i) => (
        <Skeleton key={i} width={w} />
      ))}
    </div>
  );
}
