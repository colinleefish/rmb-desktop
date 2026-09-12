import { useEffect, useState } from "react";
import { Check, Copy, X, ZoomIn } from "lucide-react";
import type { SetupArtifact } from "../../lib/agentSetupTypes";
import { useI18n } from "../../i18n";

type GuideImage = {
  src: string;
  alt: string;
};

export function RecallRuleCopy({
  artifact,
  guideImage,
  manualHint,
  contentHint,
}: {
  artifact: SetupArtifact;
  guideImage?: GuideImage;
  manualHint?: string;
  contentHint?: string;
}) {
  const { t } = useI18n();
  const [copied, setCopied] = useState(false);
  const [lightboxOpen, setLightboxOpen] = useState(false);

  useEffect(() => {
    if (!lightboxOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setLightboxOpen(false);
    };
    document.addEventListener("keydown", onKeyDown);
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.body.style.overflow = "";
    };
  }, [lightboxOpen]);

  async function handleCopy() {
    await navigator.clipboard.writeText(artifact.proposed);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <>
      <div className="overflow-hidden rounded-md border border-rmb-line bg-white">
        {manualHint && (
          <div className="space-y-1 border-b border-rmb-line bg-rmb-warn-soft px-5 py-3">
            <p className="text-xs text-rmb-warn">{manualHint}</p>
          </div>
        )}

        {!manualHint && artifact.warnings.length > 0 && (
          <div className="space-y-1 border-b border-rmb-line bg-rmb-warn-soft px-5 py-3">
            {artifact.warnings.map((w) => (
              <p key={w} className="text-xs text-rmb-warn">
                {w}
              </p>
            ))}
          </div>
        )}

        <div className="border-b border-rmb-line px-5 py-4">
          {contentHint && (
            <p className="mb-3 text-sm text-rmb-gray">{contentHint}</p>
          )}
          <pre className="m-0 overflow-auto rounded-md border border-rmb-line bg-rmb-fill p-3 font-mono text-[12px] leading-5 text-rmb-dark/90 whitespace-pre-wrap break-all">
            {artifact.proposed}
          </pre>
          <div className="mt-3">
            <button
              type="button"
              onClick={() => void handleCopy()}
              className="inline-flex items-center gap-1.5 rounded-md border border-rmb-line-strong bg-white px-3 py-1.5 text-sm font-medium text-rmb-dark hover:bg-rmb-fill"
            >
              {copied ? <Check className="size-4 text-rmb-accent" /> : <Copy className="size-4" />}
              {copied ? t.agents.copied : t.agents.copyProposed}
            </button>
          </div>
        </div>

        {guideImage && (
          <div className="px-5 py-4">
            <p className="mb-3 text-sm text-rmb-gray">{t.agents.cursorRulesGuideHint}</p>
            <button
              type="button"
              onClick={() => setLightboxOpen(true)}
              className="group relative block w-full overflow-hidden rounded-md border border-rmb-line bg-rmb-fill text-left transition hover:border-rmb-accent/40"
            >
              <img
                src={guideImage.src}
                alt={guideImage.alt}
                className="block w-full"
                loading="lazy"
              />
              <span className="absolute bottom-2 right-2 inline-flex items-center gap-1 rounded-md bg-white/95 px-2 py-1 text-xs font-medium text-rmb-dark ring-1 ring-rmb-gray/15 opacity-0 transition group-hover:opacity-100">
                <ZoomIn className="size-3.5" />
                {t.agents.clickToEnlarge}
              </span>
            </button>
          </div>
        )}
      </div>

      {lightboxOpen && guideImage && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-rmb-dark/75 p-4 sm:p-8"
          onClick={() => setLightboxOpen(false)}
          role="presentation"
        >
          <button
            type="button"
            aria-label={t.common.close}
            onClick={() => setLightboxOpen(false)}
            className="absolute right-4 top-4 rounded-md bg-white/10 p-2 text-white transition hover:bg-white/20"
          >
            <X className="size-5" />
          </button>
          <img
            src={guideImage.src}
            alt={guideImage.alt}
            className="max-h-full max-w-full rounded-lg object-contain shadow-2xl"
            onClick={(event) => event.stopPropagation()}
          />
        </div>
      )}
    </>
  );
}
