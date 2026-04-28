import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { PageShell } from "../components/ui/page-shell";
import { Card } from "../components/ui/card";
import { Badge } from "../components/ui/badge";
import { Icon, type IconName } from "../components/ui/icon";
import { Skeleton } from "../components/ui/skeleton";
import { ErrorBox } from "../components/ui/error-box";
import { EmptyState } from "../components/ui/empty-state";
import { cn } from "../lib/utils";
import { useTranslation } from "../i18n";
import type { MsgKey } from "../i18n/keys";
import { SourcesSection } from "./settings/sources-section";
import { ToolsSection } from "./settings/tools-section";
import { PromptsSection } from "./settings/prompts-section";

type NavGroup = "workspace" | "integration";

const NAV_ITEMS: {
  id: string;
  labelKey: MsgKey;
  icon: IconName;
  group: NavGroup;
}[] = [
  { id: "general", labelKey: "settingsNavGeneral", icon: "hash", group: "workspace" },
  { id: "statuses", labelKey: "settingsNavStatuses", icon: "flag", group: "workspace" },
  { id: "fields", labelKey: "settingsNavFields", icon: "filter", group: "workspace" },
  { id: "ticket-config", labelKey: "settingsNavTicketConfig", icon: "inbox", group: "workspace" },
  { id: "labels", labelKey: "settingsNavLabels", icon: "book", group: "workspace" },
  { id: "prompts", labelKey: "settingsNavPrompts", icon: "paw", group: "workspace" },
  { id: "sources", labelKey: "settingsNavSources", icon: "link", group: "integration" },
  { id: "tools", labelKey: "settingsNavTools", icon: "settings", group: "integration" },
];

const DEFAULT_SECTION = "statuses";

export default function WorkspaceSettingsPage() {
  const { workspaceId, section } = useParams<{
    workspaceId: string;
    section?: string;
  }>();
  const { t } = useTranslation();
  const validIds = new Set(NAV_ITEMS.map((i) => i.id));
  const active = section && validIds.has(section) ? section : DEFAULT_SECTION;

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

  const groups: NavGroup[] = Array.from(
    new Set(NAV_ITEMS.map((i) => i.group)),
  );
  const groupLabel = (g: NavGroup) =>
    g === "workspace"
      ? t("settingsGroupWorkspace")
      : t("settingsGroupIntegration");

  return (
    <PageShell
      crumbs={[{ label: t("settingsCrumbSettings") }]}
      showSettings
      settingsActive
    >
      <div className="max-w-[1240px] mx-auto px-8 pt-5 pb-14 grid grid-cols-[200px_1fr] gap-9">
        <nav className="sticky top-3.5 self-start">
          {groups.map((g) => (
            <div key={g}>
              <div className="text-[11px] font-semibold uppercase tracking-[0.05em] text-ink-3 px-2 pt-3 pb-1">
                {groupLabel(g)}
              </div>
              {NAV_ITEMS.filter((i) => i.group === g).map((it) => (
                <Link
                  key={it.id}
                  to={`/ws/${workspaceId}/settings/${it.id}`}
                  className={cn(
                    "w-full flex items-center gap-2 px-2.5 py-1.5 rounded-2 text-[13px] cursor-pointer text-left no-underline",
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
                  {t(it.labelKey)}
                </Link>
              ))}
            </div>
          ))}
        </nav>

        <main>
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
              title={t("settingsLoadFailed")}
              onRetry={() => refetch()}
            />
          )}

          {active === "sources" && workspaceId && (
            <SourcesSection workspaceId={workspaceId} />
          )}

          {active === "tools" && workspaceId && (
            <ToolsSection workspaceId={workspaceId} />
          )}

          {active === "prompts" && workspaceId && (
            <PromptsSection workspaceId={workspaceId} />
          )}

          {data && active === "general" && (
            <Section
              title={t("settingsTitleGeneral")}
              subtitle={t("settingsSubtitleGeneral")}
            >
              <Card className="px-4 py-3.5">
                <Row label={t("settingsRowWorkspaceId")}>
                  <code className="font-mono text-[13px]">{workspaceId}</code>
                </Row>
                <Row label={t("settingsRowDisplayName")}>
                  <span className="text-[13px]">
                    {t("settingsRowDisplayNameValue")}
                  </span>
                </Row>
              </Card>
            </Section>
          )}

          {data && active === "statuses" && (
            <Section
              title={t("settingsTitleStatuses")}
              subtitle={t("settingsSubtitleStatuses")}
            >
              <Card className="p-0 overflow-hidden">
                <table className="w-full border-separate border-spacing-0">
                  <thead>
                    <tr>
                      <Th width={240}>{t("settingsThName")}</Th>
                      <Th width={160}>{t("settingsThId")}</Th>
                      <Th width={160}>{t("settingsThColor")}</Th>
                      <Th>{t("settingsThClosed")}</Th>
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
                                {t("settingsClosedYes")}
                              </Badge>
                            ) : (
                              <span className="text-[12px] text-ink-3">
                                {t("settingsClosedNo")}
                              </span>
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
              title={t("settingsTitleFields")}
              subtitle={t("settingsSubtitleFields")}
            >
              <Card className="p-0 overflow-hidden">
                <table className="w-full border-separate border-spacing-0">
                  <thead>
                    <tr>
                      <Th>{t("settingsThName")}</Th>
                      <Th width={160}>{t("settingsThId")}</Th>
                      <Th width={140}>{t("settingsThType")}</Th>
                      <Th width={100}>{t("settingsThRequired")}</Th>
                      <Th width={100}>{t("settingsThOptions")}</Th>
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
                              {t("settingsRequired")}
                            </Badge>
                          ) : (
                            <span className="text-[12px] text-ink-3">
                              {t("settingsOptional")}
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
            <Section title={t("settingsTitleTicketConfig")}>
              <Card className="px-4 py-3.5">
                <Row label={t("settingsRowDefaultStatus")}>
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
                <Row label={t("settingsRowClosedStatuses")}>
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
                      <span className="text-[13px] text-ink-3">
                        {t("settingsClosedNone")}
                      </span>
                    )}
                  </div>
                </Row>
                <Row label={t("settingsRowAutoClose")}>
                  <span className="text-[13px] text-ink-3">
                    {t("settingsAutoCloseSoon")}
                  </span>
                </Row>
              </Card>
            </Section>
          )}

          {data && active === "labels" && (
            <Section
              title={t("settingsTitleLabels")}
              subtitle={t("settingsSubtitleLabels")}
            >
              <Card className="px-4 py-3.5">
                <Row label={t("settingsLabelTicket")}>
                  <span className="font-mono text-[13px] text-ink-1">
                    "{data.labels.ticket}"
                  </span>
                </Row>
                <Row label={t("settingsLabelTitle")}>
                  <span className="font-mono text-[13px] text-ink-1">
                    "{data.labels.title}"
                  </span>
                </Row>
                <Row label={t("settingsLabelDescription")}>
                  <span className="font-mono text-[13px] text-ink-1">
                    "{data.labels.description}"
                  </span>
                </Row>
              </Card>
            </Section>
          )}

          {active === "slack" && (
            <Section
              title={t("settingsTitleSlack")}
              subtitle={t("settingsSubtitleSlack")}
            >
              <Card className="p-2">
                <EmptyState
                  icon="slack"
                  title={t("settingsSlackEmptyTitle")}
                  description={t("settingsSlackEmptyDescription")}
                />
              </Card>
            </Section>
          )}

          {active === "members" && (
            <Section
              title={t("settingsTitleMembers")}
              subtitle={t("settingsSubtitleMembers")}
            >
              <Card className="p-2">
                <EmptyState
                  icon="user"
                  title={t("settingsMembersEmptyTitle")}
                  description={t("settingsMembersEmptyDescription")}
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
