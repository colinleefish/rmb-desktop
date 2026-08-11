import { useEffect, useRef, useState } from "react";
import { Check, ChevronDown } from "lucide-react";

export type SelectMenuOption<T extends string | number> = {
  value: T;
  label: string;
};

export function SelectMenu<T extends string | number>({
  value,
  onChange,
  options,
  id,
  labelId,
  disabled = false,
}: {
  value: T;
  onChange: (next: T) => void;
  options: SelectMenuOption<T>[];
  id?: string;
  labelId?: string;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  const current = options.find((option) => option.value === value) ?? options[0];

  useEffect(() => {
    if (!open) return;
    function handlePointerDown(event: MouseEvent) {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  function pick(next: T) {
    onChange(next);
    setOpen(false);
  }

  return (
    <div ref={rootRef} className="relative w-full">
      <button
        type="button"
        id={id}
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => !disabled && setOpen((prev) => !prev)}
        disabled={disabled}
        className={[
          "rmb-language-select flex w-full items-center justify-between gap-3 rounded border border-[#dadce0] bg-white px-3 py-[7px] text-left text-rmb-dark shadow-sm transition focus:border-rmb-accent focus:outline-none focus:ring-1 focus:ring-rmb-accent/40",
          disabled
            ? "cursor-not-allowed opacity-50"
            : "hover:border-[#bdc1c6]",
        ].join(" ")}
      >
        <span className="min-w-0 truncate">{current.label}</span>
        <ChevronDown
          className={[
            "size-4 shrink-0 text-rmb-gray/55 transition-transform",
            open ? "rotate-180" : "",
          ].join(" ")}
          aria-hidden
        />
      </button>

      {open && (
        <ul
          role="listbox"
          aria-labelledby={labelId ?? id}
          className="absolute left-0 right-0 top-[calc(100%+4px)] z-50 overflow-hidden rounded-md border border-[#dadce0] bg-white py-1 shadow-lg"
        >
          {options.map((option) => {
            const selected = option.value === value;
            return (
              <li key={String(option.value)} role="option" aria-selected={selected}>
                <button
                  type="button"
                  onClick={() => pick(option.value)}
                  className={[
                    "flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm transition",
                    selected
                      ? "bg-rmb-accent/10 text-rmb-dark"
                      : "text-rmb-dark hover:bg-rmb-light/80",
                  ].join(" ")}
                >
                  <span>{option.label}</span>
                  {selected && <Check className="size-4 shrink-0 text-rmb-accent" aria-hidden />}
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
