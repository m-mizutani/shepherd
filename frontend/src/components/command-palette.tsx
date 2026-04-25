import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "../lib/api";
import { Icon } from "./ui/icon";
import { cn } from "../lib/utils";
import { useTranslation } from "../i18n";

interface Item {
  id: string;
  label: string;
  hint?: string;
  icon?: React.ReactNode;
  onSelect: () => void;
}

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [highlight, setHighlight] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const navigate = useNavigate();
  const { workspaceId } = useParams<{ workspaceId: string }>();
  const { t } = useTranslation();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.metaKey && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
      } else if (e.key === "Escape") {
        setOpen(false);
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    if (open) {
      setQuery("");
      setHighlight(0);
      setTimeout(() => inputRef.current?.focus(), 0);
    }
  }, [open]);

  const { data: workspaces } = useQuery({
    queryKey: ["workspaces"],
    queryFn: async () => {
      const { data, error } = await api.GET("/api/v1/ws");
      if (error) throw error;
      return data;
    },
    enabled: open,
    staleTime: 60_000,
  });

  const { data: tickets } = useQuery({
    queryKey: ["tickets", workspaceId],
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/tickets",
        { params: { path: { workspaceId: workspaceId! } } },
      );
      if (error) throw error;
      return data;
    },
    enabled: open && !!workspaceId,
    staleTime: 60_000,
  });

  const items = useMemo<Item[]>(() => {
    const result: Item[] = [];
    if (workspaceId) {
      result.push({
        id: "go-tickets",
        label: t("paletteGoToTickets"),
        icon: <Icon name="inbox" size={13} />,
        onSelect: () => navigate(`/ws/${workspaceId}/tickets`),
      });
      result.push({
        id: "go-settings",
        label: t("paletteGoToSettings"),
        icon: <Icon name="settings" size={13} />,
        onSelect: () => navigate(`/ws/${workspaceId}/settings`),
      });
    }
    result.push({
      id: "go-workspaces",
      label: t("paletteSwitchWorkspace"),
      icon: <Icon name="folder" size={13} />,
      onSelect: () => navigate("/"),
    });

    workspaces?.workspaces?.forEach((w) => {
      result.push({
        id: `ws-${w.id}`,
        label: t("paletteWorkspaceLabel", { name: w.name }),
        hint: w.id,
        icon: <span className="w-2 h-2 rounded-[2px] bg-brand inline-block" />,
        onSelect: () => navigate(`/ws/${w.id}/tickets`),
      });
    });

    if (workspaceId) {
      tickets?.tickets?.forEach((tk) => {
        result.push({
          id: `t-${tk.id}`,
          label: `#${tk.seqNum} ${tk.title}`,
          hint: t("paletteOpenTicket"),
          icon: <Icon name="hash" size={13} />,
          onSelect: () => navigate(`/ws/${workspaceId}/tickets/${tk.id}`),
        });
      });
    }
    return result;
  }, [workspaceId, workspaces, tickets, navigate, t]);

  const filtered = useMemo(() => {
    if (!query.trim()) return items.slice(0, 30);
    const q = query.toLowerCase();
    return items
      .filter(
        (it) =>
          it.label.toLowerCase().includes(q) ||
          it.hint?.toLowerCase().includes(q),
      )
      .slice(0, 30);
  }, [items, query]);

  useEffect(() => {
    setHighlight(0);
  }, [query]);

  if (!open) return null;

  const select = (i: number) => {
    const it = filtered[i];
    if (!it) return;
    setOpen(false);
    it.onSelect();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center pt-[12vh] px-4 bg-black/30 backdrop-blur-[2px]"
      onMouseDown={() => setOpen(false)}
    >
      <div
        onMouseDown={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={t("paletteTitle")}
        className="bg-bg-elev border border-line rounded-4 shadow-pop w-full max-w-xl overflow-hidden"
      >
        <div className="flex items-center gap-2 px-3 py-2 border-b border-line">
          <Icon name="search" size={14} className="text-ink-4" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") {
                e.preventDefault();
                setHighlight((h) => Math.min(h + 1, filtered.length - 1));
              } else if (e.key === "ArrowUp") {
                e.preventDefault();
                setHighlight((h) => Math.max(h - 1, 0));
              } else if (e.key === "Enter") {
                e.preventDefault();
                select(highlight);
              }
            }}
            placeholder={t("palettePlaceholder")}
            className="flex-1 bg-transparent border-0 outline-none text-[14px] text-ink-1 placeholder:text-ink-4"
          />
          <span className="font-mono text-[10px] px-1.5 py-0.5 rounded-1 bg-bg-sunken border border-line text-ink-3">
            ESC
          </span>
        </div>
        <ul className="max-h-[60vh] overflow-y-auto p-1">
          {filtered.length === 0 && (
            <li className="px-3 py-6 text-center text-[12.5px] text-ink-4">
              {t("paletteEmpty")}
            </li>
          )}
          {filtered.map((it, i) => (
            <li key={it.id}>
              <button
                type="button"
                onMouseEnter={() => setHighlight(i)}
                onClick={() => select(i)}
                className={cn(
                  "w-full text-left flex items-center gap-2 px-3 py-2 rounded-2 text-[13px]",
                  i === highlight ? "bg-bg-sunken text-ink-1" : "text-ink-2",
                )}
              >
                <span className="text-ink-4">{it.icon}</span>
                <span className="flex-1 truncate">{it.label}</span>
                {it.hint && (
                  <span className="text-[11.5px] text-ink-4">{it.hint}</span>
                )}
              </button>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
