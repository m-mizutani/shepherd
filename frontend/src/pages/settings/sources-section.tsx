import { useTranslation } from "../../i18n";
import { NotionSourcesPanel } from "./sources/notion-panel";

interface Props {
  workspaceId: string;
}

// SourcesSection composes per-provider panels. Adding Slack (or anything
// else) later means writing a SlackSourcesPanel and listing it below — the
// per-provider form, columns, and empty-state stay self-contained.
export function SourcesSection({ workspaceId }: Props) {
  const { t } = useTranslation();
  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-[15px] font-semibold mb-1">{t("sourcesTitle")}</h2>
        <p className="text-[13px] text-ink-3">{t("sourcesSubtitle")}</p>
      </div>

      <NotionSourcesPanel workspaceId={workspaceId} />
      {/* Future: <SlackSourcesPanel workspaceId={workspaceId} /> */}
    </div>
  );
}
