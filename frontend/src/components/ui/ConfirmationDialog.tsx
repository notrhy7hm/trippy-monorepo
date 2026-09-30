import { createContext, useCallback, useContext, useEffect, useId, useRef, useState } from "react";
import type { ReactNode } from "react";
import { createPortal } from "react-dom";
import { useLocation } from "react-router-dom";
import { Trash2, X } from "lucide-react";
import { Button } from "./Button";

type Confirmation = { title: string; description: string; confirmLabel?: string };
const ConfirmationContext = createContext<((options: Confirmation) => Promise<boolean>) | null>(null);

export function ConfirmationProvider({ children }: { children: ReactNode }) {
  const [pending, setPending] = useState<Confirmation | null>(null);
  const resolver = useRef<((confirmed: boolean) => void) | null>(null);
  const trigger = useRef<HTMLElement | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  const id = useId();
  const location = useLocation();

  const settle = useCallback((confirmed: boolean) => {
    const resolve = resolver.current;
    resolver.current = null;
    dialog.current?.close();
    setPending(null);
    resolve?.(confirmed);
    if (trigger.current?.isConnected) trigger.current.focus();
  }, []);

  const confirm = useCallback((options: Confirmation) => {
    resolver.current?.(false);
    trigger.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve;
      setPending(options);
    });
  }, []);

  useEffect(() => { settle(false); }, [location.key, settle]);
  useEffect(() => () => { resolver.current?.(false); resolver.current = null; }, []);
  useEffect(() => {
    if (pending && dialog.current && !dialog.current.open) {
      dialog.current.showModal();
      cancel.current?.focus();
    }
  }, [pending]);

  return (
    <ConfirmationContext.Provider value={confirm}>
      {children}
      {pending && createPortal(
        <dialog
          ref={dialog}
          role="alertdialog"
          aria-modal="true"
          aria-labelledby={`${id}-title`}
          aria-describedby={`${id}-description`}
          onCancel={(event) => { event.preventDefault(); settle(false); }}
          onClick={(event) => {
            if (event.target !== event.currentTarget) return;
            const rect = event.currentTarget.getBoundingClientRect();
            if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) settle(false);
          }}
          className="fixed inset-0 m-auto w-[calc(100%-2rem)] max-w-md rounded-lg border border-ink-200 bg-white p-0 text-ink-900 shadow-xl backdrop:bg-black/40"
        >
          <div className="p-5 sm:p-6">
            <div className="flex items-center justify-between gap-4">
              <h2 id={`${id}-title`} className="text-lg font-semibold">{pending.title}</h2>
              <button type="button" aria-label="Close confirmation" title="Close" onClick={() => settle(false)} className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-ink-500 hover:bg-ink-100"><X size={18} /></button>
            </div>
            <p id={`${id}-description`} className="mt-3 break-words text-sm leading-6 text-ink-600">{pending.description}</p>
            <div className="mt-6 flex justify-end gap-2">
              <button ref={cancel} type="button" onClick={() => settle(false)} className="rounded-md border border-ink-200 px-4 py-2 text-sm font-medium hover:bg-ink-50 focus:outline-none focus:ring-2 focus:ring-ink-300">Cancel</button>
              <Button type="button" onClick={() => settle(true)} className="gap-2 bg-red-600 text-white hover:bg-red-700"><Trash2 size={16} />{pending.confirmLabel ?? "Delete"}</Button>
            </div>
          </div>
        </dialog>, document.body,
      )}
    </ConfirmationContext.Provider>
  );
}

export function useConfirmation() {
  const confirm = useContext(ConfirmationContext);
  if (!confirm) throw new Error("useConfirmation must be used within ConfirmationProvider");
  return confirm;
}
