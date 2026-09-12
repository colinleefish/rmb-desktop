import { useState } from "react";

function maskedKeyLabel(suffix: string): string {
  return `${"•".repeat(12)}${suffix}`;
}

export function ConfiguredApiKeyField({
  label,
  configured,
  keySuffix,
  value,
  onChange,
  emptyPlaceholder,
  replacePlaceholder,
}: {
  label: string;
  configured: boolean;
  keySuffix?: string;
  value: string;
  onChange: (v: string) => void;
  emptyPlaceholder: string;
  replacePlaceholder: string;
}) {
  const [editing, setEditing] = useState(false);
  const hasStoredKey = configured && !!keySuffix;
  const showMasked = hasStoredKey && !editing && !value;

  return (
    <div>
      <label className="block text-sm font-medium text-rmb-gray">{label}</label>
      <input
        type={editing || value ? "password" : "text"}
        value={showMasked ? maskedKeyLabel(keySuffix) : value}
        readOnly={showMasked}
        onFocus={() => {
          setEditing(true);
          if (hasStoredKey && !value) {
            onChange("");
          }
        }}
        onBlur={() => setEditing(false)}
        onChange={(e) => {
          if (showMasked) return;
          onChange(e.target.value);
        }}
        placeholder={configured ? replacePlaceholder : emptyPlaceholder}
        aria-label={label}
        autoComplete="off"
        className="mt-1 w-full rounded-md border border-rmb-line-strong h-8 px-3 font-mono text-sm outline-none transition-colors focus:border-rmb-accent placeholder:font-sans placeholder:text-rmb-faint"
      />
    </div>
  );
}
