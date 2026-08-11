const MASKED_KEY_DISPLAY = "••••••••••••••••";

export function ConfiguredApiKeyField({
  label,
  configured,
  value,
  onChange,
  emptyPlaceholder,
  replacePlaceholder,
}: {
  label: string;
  configured: boolean;
  value: string;
  onChange: (v: string) => void;
  emptyPlaceholder: string;
  replacePlaceholder: string;
}) {
  const showMasked = configured && !value;

  return (
    <div>
      <label className="block text-sm font-medium text-rmb-gray">{label}</label>
      {showMasked && (
        <div
          className="mt-1 rounded-md border border-rmb-gray/20 bg-rmb-light/40 px-3 py-2 font-mono text-sm tracking-[0.15em] text-rmb-dark"
          aria-hidden
        >
          {MASKED_KEY_DISPLAY}
        </div>
      )}
      <input
        type="password"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={configured ? replacePlaceholder : emptyPlaceholder}
        aria-label={configured ? `${label} (${replacePlaceholder})` : label}
        className={`w-full rounded-md border border-rmb-gray/20 px-3 py-2 text-sm placeholder:text-rmb-gray/45 ${
          showMasked ? "mt-2" : "mt-1"
        }`}
      />
    </div>
  );
}
