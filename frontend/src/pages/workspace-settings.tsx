import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router-dom";
import { useState } from "react";
import { api } from "../lib/api";
import { PageShell } from "../components/ui/page-shell";
import { Card } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Icon, type IconName } from "../components/ui/icon";
import { Skeleton } from "../components/ui/skeleton";
import { ErrorBox } from "../components/ui/error-box";
import { EmptyState } from "../components/ui/empty-state";
import { cn } from "../lib/utils";

const NAV_ITEMS: {
  id: string;
  label: string;
  icon: IconName;
  group: "Workspace" | "Integration";
}[] = [
  { id: "general", label: "General", icon: "hash", group: "Workspace" },
  { id: "statuses", label: "Statuses", icon: "flag", group: "Workspace" },
  { id: "fields", label: "Fields", icon: "filter", group: "Workspace" },
  { id: "ticket-config", label: "Ticket Config", icon: "inbox", group: "Workspace" },
  { id: "labels", label: "Labels", icon: "book", group: "Workspace" },
  { id: "slack", label: "Slack", icon: "slack", group: "Integration" },
  { id: "members", label: "Members", icon: "user", group: "Integration" },
];

export default function WorkspaceSettingsPage() {
  const { workspaceId } = useParams<{ workspaceId: string }>();
  const [active, setActive] = useState<string>("statuses");
  const [editMode, setEditMode] = useState(false);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["workspace-config", workspaceId],
    queryFn: async () => {
      const { data, error } = await api.GET(
        "/api/v1/ws/{workspaceId}/config",
        { params: { path: { workspaceId: workspaceId! } } },
      );
      if (error) throw error;
      return data;
    },
  });

  const groups = Array.from(new Set(NAV_ITEMS.map((i) => i.group)));

  return (
    <PageShell
      crumbs={[{ label: "Settings" }]}
      showSettings
      settingsActive
    >
      <div className="max-w-[1240px] mx-auto px-8 pt-5 pb-14 grid grid-cols-[200px_1fr] gap-9">
        <nav className="sticky top-3.5 self-start">
          {groups.map((g) => (
            <div key={g}>
              <div className="text-[11px] font-semibold uppercase tracking-[0.05em] text-ink-3 px-2 pt-3 pb-1">
                {g}
              </div>
              {NAV_ITEMS.filter((i) => i.group === g).map((it) => (
                <button
                  key={it.id}
                  type="button"
                  onClick={() => setActive(it.id)}
                  className={cn(
                    "w-full flex items-center gap-2 px-2.5 py-1.5 rounded-2 text-[13px] cursor-pointer text-left",
                    it.id === active
                      ? "bg-brand-soft text-ink-1 font-semibold"
                      : "text-ink-3 font-medium hover:bg-bg-sunken hover:text-ink-1",
                  )}
                >
                  <Icon
                    name={it.icon}
                    size={13}
                    className={
                      it.id === active ? "text-brand" : "text-ink-4"
                    }
                  />
                  {it.label}
                </button>
              ))}
            </div>
          ))}
        </nav>

        <main>
          {/* Read-only banner */}
          <div className="px-3.5 py-2.5 mb-5 bg-info-soft border border-[#bfdbfe] rounded-3 flex items-center gap-2.5">
            <Icon name="eye" size={14} className="text-info" />
            <div className="flex-1 text-[12.5px] text-[#1e3a8a]">
              {editMode ? (
                <>Edit mode is on. Most fields are still managed in <code className="bg-white px-1.5 py-px rounded-1 border border-[#bfdbfe] font-mono">workspace.yaml</code>.</>
              ) : (
                <>Read-only — workspace settings are managed in <code className="bg-white px-1.5 py-px rounded-1 border border-[#bfdbfe] font-mono">workspace.yaml</code> for now. Editing UI is coming soon.</>
              )}
            </div>
            <Button
              size="sm"
              onClick={() => setEditMode((m) => !m)}
            >
              <Icon name="edit" size={11} /> {editMode ? "Done" : "Edit"}
            </Button>
          </div>

          {isLoading && (
            <div className="space-y-3">
              <Skeleton width="40%" height={18} />
              <Card className="p-4 space-y-3">
                <Skeleton width="60%" />
                <Skeleton width="80%" />
                <Skeleton width="50%" />
              </Card>
            </div>
          )}

          {error && (
            <ErrorBox
              title="Failed to load workspace config"
              onRetry={() => refetch()}
            />
          )}

          {data && active === "general" && (
            <Section title="General" subtitle="Basic workspace metadata.">
              <Card className="px-4 py-3.5">
                <Row label="Workspace ID">
                  <code className="font-mono text-[13px]">{workspaceId}</code>
                </Row>
                <Row label="Display name">
                  <span className="text-[13px]">
                    Defined in <code className="font-mono">workspace.yaml</code>
                  </span>
                </Row>
              </Card>
            </Section>
          )}

          {data && active === "statuses" && (
            <Section
              title="Statuses"
              subtitle="Lifecycle stages a ticket flows through. The chosen color is used for badges and chart segments."
              action={
                <Button size="sm" disabled={!editMode}>
                  <Icon name="plus" size={11} /> Add
                </Button>
              }
            >
              <Card className="p-0 overflow-hidden">
                <table className="w-full border-separate border-spacing-0">
                  <thead>
                    <tr>
                      <Th width={240}>Name</Th>
                      <Th width={160}>ID</Th>
                      <Th width={160}>Color</Th>
                      <Th>Closed?</Th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.statuses.map((s) => {
                      const closed =
                        data.ticketConfig.closedStatusIds.includes(s.id);
                      return (
                        <tr key={s.id} className="[&_td]:border-b [&_td]:border-line last:[&_td]:border-b-0">
                          <Td>
                            <Badge color={s.color}>{s.name}</Badge>
                          </Td>
                          <Td className="font-mono text-ink-3">{s.id}</Td>
                          <Td>
                            <div className="inline-flex items-center gap-1.5">
                              <span
                                className="w-3.5 h-3.5 rounded-1 border border-black/10"
                                style={{ background: s.color }}
                              />
                              <span className="font-mono text-ink-3">
                                {s.color}
                              </span>
                            </div>
                          </Td>
                          <Td>
                            {closed ? (
                              <Badge tone="neutral" dot={false}>
                                Yes
                              </Badge>
                            ) : (
                              <span className="text-[12px] text-ink-3">No</span>
                            )}
                          </Td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </Card>
            </Section>
          )}

          {data && active === "fields" && (
            <Section
              title="Custom Fields"
              subtitle="Per-workspace metadata attached to every ticket."
              action={
                <Button size="sm" disabled={!editMode}>
                  <Icon name="plus" size={11} /> Add
                </Button>
              }
            >
              <Card className="p-0 overflow-hidden">
                <table className="w-full border-separate border-spacing-0">
                  <thead>
                    <tr>
                      <Th>Name</Th>
                      <Th width={160}>ID</Th>
                      <Th width={140}>Type</Th>
                      <Th width={100}>Required</Th>
                      <Th width={100}>Options</Th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.fields.map((f) => (
                      <tr key={f.id} className="[&_td]:border-b [&_td]:border-line last:[&_td]:border-b-0">
                        <Td className="font-medium">{f.name}</Td>
                        <Td className="font-mono text-ink-3">{f.id}</Td>
                        <Td>
                          <span className="inline-flex items-center px-2 h-[22px] rounded-full bg-bg-sunken text-ink-2 text-[11.5px] font-medium">
                            {f.type}
                          </span>
                        </Td>
                        <Td>
                          {f.required ? (
                            <Badge tone="danger" dot={false}>
                              Required
                            </Badge>
                          ) : (
                            <span className="text-[12px] text-ink-3">
                              Optional
                            </span>
                          )}
                        </Td>
                        <Td className="font-mono text-ink-3">
                          {f.options?.length ?? "—"}
                        </Td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </Card>
            </Section>
          )}

          {data && active === "ticket-config" && (
            <Section title="Ticket Config">
              <Card className="px-4 py-3.5">
                <Row label="Default Status">
                  {(() => {
                    const s = data.statuses.find(
                      (x) => x.id === data.ticketConfig.defaultStatusId,
                    );
                    return s ? (
                      <Badge color={s.color}>{s.name}</Badge>
                    ) : (
                      <span className="text-[13px] text-ink-3">
                        {data.ticketConfig.defaultStatusId}
                      </span>
                    );
                  })()}
                </Row>
                <Row label="Closed Statuses">
                  <div className="flex gap-1.5">
                    {data.ticketConfig.closedStatusIds.map((id) => {
                      const s = data.statuses.find((x) => x.id === id);
                      return s ? (
                        <Badge key={id} color={s.color}>
                          {s.name}
                        </Badge>
                      ) : (
                        <span key={id} className="text-[13px]">
                          {id}
                        </span>
                      );
                    })}
                    {data.ticketConfig.closedStatusIds.length === 0 && (
                      <span className="text-[13px] text-ink-3">None</span>
                    )}
                  </div>
                </Row>
                <Row label="Auto-close after">
                  <span className="text-[13px] text-ink-3">
                    Coming soon — see{" "}
                    <a
                      href="https://github.com/m-mizutani/shepherd/issues/9"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-info underline"
                    >
                      #9
                    </a>
                  </span>
                </Row>
              </Card>
            </Section>
          )}

          {data && active === "labels" && (
            <Section title="Labels" subtitle="Strings shown in the UI. Used for i18n.">
              <Card className="px-4 py-3.5">
                <Row label="Ticket">
                  <span className="font-mono text-[13px] text-ink-1">
                    "{data.labels.ticket}"
                  </span>
                </Row>
                <Row label="Title">
                  <span className="font-mono text-[13px] text-ink-1">
                    "{data.labels.title}"
                  </span>
                </Row>
                <Row label="Description">
                  <span className="font-mono text-[13px] text-ink-1">
                    "{data.labels.description}"
                  </span>
                </Row>
              </Card>
            </Section>
          )}

          {active === "slack" && (
            <Section
              title="Slack integration"
              subtitle="Channel mappings and bot-user configuration."
            >
              <Card className="p-2">
                <EmptyState
                  icon="slack"
                  title="Slack integration view"
                  description={
                    <>
                      Channel-level configuration is read from{" "}
                      <code className="font-mono">workspace.yaml</code>. A
                      dedicated UI is tracked in{" "}
                      <a
                        href="https://github.com/m-mizutani/shepherd/issues/8"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-info underline"
                      >
                        #8
                      </a>
                      .
                    </>
                  }
                />
              </Card>
            </Section>
          )}

          {active === "members" && (
            <Section
              title="Members"
              subtitle="Slack users with access to this workspace."
            >
              <Card className="p-2">
                <EmptyState
                  icon="user"
                  title="Members listing"
                  description={
                    <>
                      Coming with{" "}
                      <a
                        href="https://github.com/m-mizutani/shepherd/issues/8"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-info underline"
                      >
                        #8
                      </a>
                      .
                    </>
                  }
                />
              </Card>
            </Section>
          )}
        </main>
      </div>
    </PageShell>
  );
}

function Section({
  title,
  subtitle,
  action,
  children,
}: {
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className="mb-7">
      <div className="flex items-end justify-between mb-2.5">
        <div>
          <h2 className="m-0 text-[18px] font-semibold tracking-[-0.012em] text-ink-1">
            {title}
          </h2>
          {subtitle && (
            <div className="text-[12px] text-ink-3 mt-0.5">{subtitle}</div>
          )}
        </div>
        {action}
      </div>
      {children}
    </section>
  );
}

function Th({ children, width }: { children: React.ReactNode; width?: number }) {
  return (
    <th
      style={{ width }}
      className="text-left py-2.5 px-3.5 text-[11.5px] font-medium uppercase tracking-[0.04em] text-ink-4 bg-bg-elev border-b border-line"
    >
      {children}
    </th>
  );
}

function Td({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <td
      className={cn(
        "py-3 px-3.5 text-[13px] align-middle text-ink-1",
        className,
      )}
    >
      {children}
    </td>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[200px_1fr] gap-3.5 py-1.5 items-center">
      <div className="text-[12px] font-medium text-ink-3">{label}</div>
      <div>{children}</div>
    </div>
  );
}
