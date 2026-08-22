import { X } from 'lucide-react';
import { useEffect, useRef, type ReactNode } from 'react';

export function Modal({ open, title, children, footer, onClose }: { open: boolean; title: string; children: ReactNode; footer?: ReactNode; onClose: () => void }) {
  const dialog = useRef<HTMLDivElement>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  useEffect(() => {
    if (!open) return;
    previousFocus.current = document.activeElement as HTMLElement;
    const timer = window.setTimeout(() => dialog.current?.querySelector<HTMLElement>('button, input, select, textarea')?.focus(), 0);
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
      if (event.key !== 'Tab' || !dialog.current) return;
      const nodes = [...dialog.current.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])')];
      if (!nodes.length) return;
      const first = nodes[0]; const last = nodes[nodes.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    };
    document.addEventListener('keydown', keydown);
    return () => { window.clearTimeout(timer); document.removeEventListener('keydown', keydown); previousFocus.current?.focus(); };
  }, [onClose, open]);
  if (!open) return null;
  return <div className="modal-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
    <div className="modal" role="dialog" aria-modal="true" aria-labelledby="modal-title" ref={dialog}>
      <header><h2 id="modal-title">{title}</h2><button className="icon-button" onClick={onClose} aria-label="Close"><X /></button></header>
      <div className="modal-body">{children}</div>
      {footer && <footer>{footer}</footer>}
    </div>
  </div>;
}
