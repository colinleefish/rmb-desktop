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
        className="absolute inset-0 bg-rmb-dark/30"
        onClick={onClose}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
        className="relative z-10 flex max-h-[min(85vh,720px)] w-full max-w-2xl flex-col overflow-hidden rounded-lg border border-rmb-line bg-white shadow-rmb-modal"
      >
        <div className="flex items-start justify-between gap-4 border-b border-rmb-line px-6 py-4">
          <div className="min-w-0">
            <h2 id="modal-title" className="truncate text-base font-semibold text-rmb-dark">
              {title}
            </h2>
            {subtitle && (
              <p className="mt-0.5 truncate font-mono text-xs text-rmb-faint">{subtitle}</p>
            )}
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="rounded-md p-1 text-rmb-muted transition-colors hover:bg-rmb-fill hover:text-rmb-dark"
          >
            <X className="size-4" />
          </button>
        </div>
        <div className="overflow-y-auto px-6 py-5">{children}</div>
      </div>
    </div>
  );
}
