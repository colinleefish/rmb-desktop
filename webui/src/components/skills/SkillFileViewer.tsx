export function SkillFileViewer({
  path,
  content,
}: {
  path: string | null;
  content: string;
}) {
  if (!path) {
    return <p className="text-sm text-rmb-gray">Select a file to preview.</p>;
  }

  const isMarkdown = path.endsWith(".md");

  return (
    <div className="max-h-[min(70vh,640px)] overflow-auto rounded-xl border border-rmb-gray/20 bg-rmb-light/30 p-4">
      <div className="mb-3 font-mono text-xs text-rmb-gray">{path}</div>
      {isMarkdown ? (
        <pre className="whitespace-pre-wrap text-sm leading-relaxed text-rmb-dark">{content}</pre>
      ) : (
        <pre className="whitespace-pre-wrap font-mono text-xs leading-relaxed text-rmb-dark">
          {content}
        </pre>
      )}
    </div>
  );
}
