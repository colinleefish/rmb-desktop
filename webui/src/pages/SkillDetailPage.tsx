import { useEffect, useMemo, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { SkillFileTree } from "../components/skills/SkillFileTree";
import { SkillFileViewer } from "../components/skills/SkillFileViewer";
import { getSkill } from "../lib/api";
import { formatDateTime } from "../lib/format";
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
    return <p className="text-red-600">{t.skills.loadError}: {error}</p>;
  }

  if (loading || !detail) {
    return <p className="text-rmb-gray">{t.skills.loadingDetail}</p>;
  }

  const content = selectedFile ? (detail.files[selectedFile] ?? "") : "";

  return (
    <div className="space-y-6">
      <div>
        <Link to="/skills" className="text-sm text-rmb-accent hover:underline">
          ← {t.skills.backToList}
        </Link>
      </div>

      <div className="space-y-2 border-b border-rmb-gray/15 pb-4">
        <h1 className="text-2xl font-semibold tracking-tight text-rmb-dark">
          {detail.skill.name}
        </h1>
        <p className="max-w-3xl text-sm text-rmb-gray">{detail.skill.description}</p>
        <div className="flex flex-wrap gap-4 text-xs text-rmb-gray">
          <span>v{detail.skill.version}</span>
          <span>{formatDateTime(detail.skill.updated_at)}</span>
          <span className="font-mono">{detail.skill.uri}</span>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[240px_minmax(0,1fr)]">
        <div className="rounded-xl border border-rmb-gray/20 bg-white p-2">
          <SkillFileTree
            tree={detail.tree}
            selected={selectedFile}
            onSelect={setFile}
          />
        </div>
        <SkillFileViewer path={selectedFile} content={content} />
      </div>
    </div>
  );
}

function NavigateToSkills() {
  return (
    <p className="text-rmb-gray">
      <Link to="/skills" className="text-rmb-accent hover:underline">
        Back to skills
      </Link>
    </p>
  );
}
