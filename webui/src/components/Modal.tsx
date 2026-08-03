import { useEffect, type ReactNode } from "react";
import { X } from "lucide-react";

type ModalProps = {
  open: boolean;
  onClose: () => void;
  title: string;
  subtitle?: string;
  children: ReactNode;
};

export function Modal({ open, onClose, title, subtitle, children }: ModalProps) {
  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.body.style.overflow = "";
    };
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <button
        type="button"
        aria-label="Close"
        className="absolute inset-0 bg-rmb-dark/40"
        onClick={onClose}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
        className="relative z-10 flex max-h-[min(85vh,720px)] w-full max-w-2xl flex-col overflow-hidden rounded-xl border border-rmb-gray/20 bg-white shadow-xl"
      >
        <div className="flex items-start justify-between gap-4 border-b border-rmb-gray/15 px-6 py-4">
          <div className="min-w-0">
            <h2 id="modal-title" className="truncate text-lg font-semibold text-rmb-dark">
              {title}
            </h2>
            {subtitle && <p className="mt-1 truncate text-xs text-rmb-gray">{subtitle}</p>}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-rmb-gray transition hover:bg-rmb-light hover:text-rmb-dark"
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="overflow-y-auto px-6 py-4">{children}</div>
      </div>
    </div>
  );
}
