import { type ReactNode, useMemo } from "react";

function parseInline(text: string, keyPrefix: string): ReactNode[] {
  const parts = text.split(/(\*\*[^*]+\*\*)/g);
  return parts.map((part, index) => {
    if (part.startsWith("**") && part.endsWith("**") && part.length > 4) {
      return (
        <strong key={`${keyPrefix}-b-${index}`} className="font-semibold text-rmb-dark">
          {part.slice(2, -2)}
        </strong>
      );
    }
    return part;
  });
}

function parseProfileMarkdown(content: string): ReactNode[] {
  const lines = content.replace(/\r\n/g, "\n").split("\n");
  const blocks: ReactNode[] = [];
  let index = 0;
  let blockKey = 0;

  while (index < lines.length) {
    const line = lines[index];
    const trimmed = line.trim();

    if (trimmed === "") {
      index += 1;
      continue;
    }

    if (trimmed.startsWith("# ")) {
      blocks.push(
        <h2
          key={`h-${blockKey++}`}
          className="mt-9 border-b border-rmb-line pb-2 text-base font-semibold tracking-tight text-rmb-dark"
        >
          {parseInline(trimmed.slice(2), `h-${blockKey}`)}
        </h2>,
      );
      index += 1;
      continue;
    }

    if (trimmed.startsWith("## ")) {
      blocks.push(
        <h3
          key={`h2-${blockKey++}`}
          className="mt-8 text-sm font-semibold tracking-tight text-rmb-dark"
        >
          {parseInline(trimmed.slice(3), `h2-${blockKey}`)}
        </h3>,
      );
      index += 1;
      continue;
    }

    if (trimmed.startsWith("- ")) {
      const items: string[] = [];
      while (index < lines.length && lines[index].trim().startsWith("- ")) {
        items.push(lines[index].trim().slice(2));
        index += 1;
      }
      blocks.push(
        <ul
          key={`ul-${blockKey++}`}
          className="mt-3 list-disc space-y-2 pl-5 text-sm leading-relaxed text-rmb-muted"
        >
          {items.map((item, itemIndex) => (
            <li key={itemIndex} className="pl-0.5">
              {parseInline(item, `li-${blockKey}-${itemIndex}`)}
            </li>
          ))}
        </ul>,
      );
      continue;
    }

    const paragraphLines: string[] = [];
    while (index < lines.length) {
      const next = lines[index].trim();
      if (next === "" || next.startsWith("# ") || next.startsWith("## ") || next.startsWith("- ")) {
        break;
      }
      paragraphLines.push(next);
      index += 1;
    }
    blocks.push(
      <p key={`p-${blockKey++}`} className="mt-3 text-sm leading-relaxed text-rmb-muted">
        {parseInline(paragraphLines.join(" "), `p-${blockKey}`)}
      </p>,
    );
  }

  return blocks;
}

export function MemoryMarkdown({ content }: { content: string }) {
  const blocks = useMemo(() => parseProfileMarkdown(content), [content]);
  return <div className="memory-markdown [&>:first-child]:mt-0">{blocks}</div>;
}
