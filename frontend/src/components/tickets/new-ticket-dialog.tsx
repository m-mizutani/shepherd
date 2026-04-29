import { useEffect, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { Button } from "../ui/button";
import { Dialog } from "../ui/dialog";
import { ErrorBox } from "../ui/error-box";
import { useTranslation } from "../../i18n";

export function NewTicketDialog({
  open,
  onClose,
  workspaceId,
  defaultStatusId,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  workspaceId: string;
  defaultStatusId?: string;
  onCreated: (ticketId: string) => void;
}) {
  const { t } = useTranslation();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const create = useMutation({
    mutationFn: async () => {
      const { data, error } = await api.POST(
        "/api/v1/ws/{workspaceId}/tickets",
        {
          params: { path: { workspaceId } },
          body: {
            title,
            description: description || undefined,
            statusId: defaultStatusId || undefined,
          },
        },
      );
      if (error) throw error;
      return data!;
    },
    onSuccess: (ticket) => {
      setTitle("");
      setDescription("");
      onCreated(ticket.id);
    },
  });

  useEffect(() => {
    if (!open) {
      setTitle("");
      setDescription("");
      create.reset();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={t("ticketDialogNewTitle")}
      width={520}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            {t("btnCancel")}
          </Button>
          <Button
            variant="primary"
            disabled={!title.trim() || create.isPending}
            onClick={() => create.mutate()}
          >
            {create.isPending
              ? t("ticketDialogCreating")
              : t("ticketDialogCreate")}
          </Button>
        </>
      }
    >
      <div className="space-y-3">
        <label className="block">
          <span className="text-[12px] font-medium text-ink-3">
            {t("ticketDialogTitleLabel")}
          </span>
          <input
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            autoFocus
            placeholder={t("ticketDialogTitlePlaceholder")}
            className="mt-1 w-full h-9 px-3 bg-bg-elev border border-line-strong rounded-2 text-[13.5px] text-ink-1 focus:outline-none focus:border-brand focus:ring-2 focus:ring-brand-soft"
          />
        </label>
        <label className="block">
          <span className="text-[12px] font-medium text-ink-3">
            {t("ticketDialogDescriptionLabel")}
          </span>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={5}
            placeholder={t("ticketDialogDescriptionPlaceholder")}
            className="mt-1 w-full px-3 py-2 bg-bg-elev border border-line-strong rounded-2 text-[13.5px] text-ink-1 focus:outline-none focus:border-brand focus:ring-2 focus:ring-brand-soft"
          />
        </label>
        {create.isError && (
          <ErrorBox
            title={t("ticketDialogCreateFailed")}
            onRetry={() => create.mutate()}
          />
        )}
      </div>
    </Dialog>
  );
}
