import type { SkillFileNode } from "../../lib/types";

function TreeNode({
  node,
  selected,
  onSelect,
  depth = 0,
}: {
  node: SkillFileNode;
  selected: string | null;
  onSelect: (path: string) => void;
  depth?: number;
}) {
  const isDir = node.type === "dir";
  const isSelected = !isDir && node.path === selected;

  return (
    <div>
      <button
        type="button"
        onClick={() => !isDir && onSelect(node.path)}
        className={[
          "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm",
          isDir ? "cursor-default text-rmb-gray" : "text-rmb-dark hover:bg-rmb-light",
          isSelected ? "bg-rmb-accent/10 font-medium text-rmb-accent" : "",
        ].join(" ")}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
        disabled={isDir}
      >
        <span className="truncate">{node.name}</span>
      </button>
      {node.children?.map((child) => (
        <TreeNode
          key={child.path}
          node={child}
          selected={selected}
          onSelect={onSelect}
          depth={depth + 1}
        />
      ))}
    </div>
  );
}

export function SkillFileTree({
  tree,
  selected,
  onSelect,
}: {
  tree: SkillFileNode[];
  selected: string | null;
  onSelect: (path: string) => void;
}) {
  return (
    <div className="space-y-0.5">
      {tree.map((node) => (
        <TreeNode
          key={node.path}
          node={node}
          selected={selected}
          onSelect={onSelect}
        />
      ))}
    </div>
  );
}
