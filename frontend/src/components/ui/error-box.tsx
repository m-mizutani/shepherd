import { Button } from "./button";
import { Icon } from "./icon";

interface Props {
  title?: string;
  message?: string;
  onRetry?: () => void;
}

export function ErrorBox({ title = "Something went wrong", message, onRetry }: Props) {
  return (
    <div className="flex items-start gap-3 p-4 rounded-3 border border-danger/20 bg-danger-soft">
      <span className="text-danger mt-0.5">
        <Icon name="alert" size={16} />
      </span>
      <div className="flex-1 min-w-0">
        <div className="text-[13px] font-semibold text-danger">{title}</div>
        {message && (
          <div className="text-[12.5px] text-ink-2 mt-0.5">{message}</div>
        )}
      </div>
      {onRetry && (
        <Button size="sm" variant="default" onClick={onRetry}>
          <Icon name="refresh" size={12} /> Retry
        </Button>
      )}
    </div>
  );
}
