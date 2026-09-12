import { useEffect, useMemo, useState } from "react";
import { useParams, useSearchParams } from "react-router-dom";
import { EmptyState, ErrorNote, ListSkeleton } from "../components/EmptyState";
import { PageHeader } from "../components/PageHeader";
import { StatusPill } from "../components/StatusPill";
import { SkillFileTree } from "../components/skills/SkillFileTree";
import { SkillFileViewer } from "../components/skills/SkillFileViewer";
import { getSkill } from "../lib/api";
import { formatDateTime, formatDateTimeMonoClass } from "../lib/format";
import type { SkillDetail } from "../lib/types";
import { useI18n } from "../i18n";

function defaultFile(detail: SkillDetail | null): string | null {
  if (!detail) return null;
  if (detail.files["SKILL.md"]) return "SKILL.md";
  const paths = Object.keys(detail.files).sort();
  return paths[0] ?? null;
}

export function SkillDetailPage() {
  const { t } = useI18n();
  const { slug = "" } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const fileParam = searchParams.get("file");

  const [detail, setDetail] = useState<SkillDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!slug) return;
    setLoading(true);
    setError(null);
    setDetail(null);
    getSkill(slug)
      .then(setDetail)
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false));
  }, [slug]);

  const selectedFile = useMemo(() => {
    if (fileParam && detail?.files[fileParam] !== undefined) return fileParam;
    return defaultFile(detail);
  }, [fileParam, detail]);

  const setFile = (path: string) => {
    setSearchParams({ file: path }, { replace: true });
  };

  if (!slug) {
    return <NavigateToSkills />;
  }

  if (error) {
    return (
      <ErrorNote>
        {t.skills.loadError}: {error}
      </ErrorNote>
    );
  }

  if (loading || !detail) return <ListSkeleton rows={4} />;

  const content = selectedFile ? (detail.files[selectedFile] ?? "") : "";

  return (
    <div className="space-y-6">
      <PageHeader
        back={{ to: "/agents/skills", label: t.skills.backToList }}
        title={detail.skill.name}
        description={detail.skill.description}
        meta={
          <>
            {(detail.skill.tags ?? []).map((tag) => (
              <StatusPill key={tag} tone="neutral">
                {tag}
              </StatusPill>
            ))}
            <span>v{detail.skill.version}</span>
            <span className={formatDateTimeMonoClass}>{formatDateTime(detail.skill.updated_at)}</span>
            <span className="font-mono text-rmb-faint">{detail.skill.uri}</span>
          </>
        }
      />

      <div className="grid gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
        <div className="rounded-md border border-rmb-line p-2">
          <SkillFileTree tree={detail.tree} selected={selectedFile} onSelect={setFile} />
        </div>
        <SkillFileViewer path={selectedFile} content={content} />
      </div>
    </div>
  );
}

function NavigateToSkills() {
  const { t } = useI18n();
  return (
    <EmptyState
      title={t.skills.empty}
      action={{ to: "/agents/skills", label: t.skills.backToList }}
    />
  );
}
