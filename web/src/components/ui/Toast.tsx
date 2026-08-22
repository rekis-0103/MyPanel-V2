import { AlertCircle, CheckCircle2, Info, TriangleAlert, X } from 'lucide-react';
import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react';

type ToastVariant = 'success' | 'error' | 'info' | 'warning';
type ToastItem = { id: number; message: string; variant: ToastVariant; closing?: boolean };
type ToastAPI = { showToast: (message: string, variant?: ToastVariant) => void };
const ToastContext = createContext<ToastAPI | null>(null);
let nextToastId = 1;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const dismiss = useCallback((id: number) => setItems((current) => current.filter((item) => item.id !== id)), []);
  const showToast = useCallback((message: string, variant: ToastVariant = 'info') => {
    const id = nextToastId++;
    setItems((current) => [...current, { id, message, variant }]);
    window.setTimeout(() => setItems((current) => current.map((item) => item.id === id ? { ...item, closing: true } : item)), 3800);
    window.setTimeout(() => dismiss(id), 4000);
  }, [dismiss]);
  const value = useMemo(() => ({ showToast }), [showToast]);
  const icons = { success: CheckCircle2, error: AlertCircle, info: Info, warning: TriangleAlert };
  return <ToastContext.Provider value={value}>{children}<div className="toast-region" aria-live="polite">
    {items.map((item) => { const Icon = icons[item.variant]; return <div key={item.id} className={`toast ${item.variant} ${item.closing ? 'closing' : ''}`}><Icon /><span>{item.message}</span><button onClick={() => dismiss(item.id)} aria-label="Close"><X /></button></div>; })}
  </div></ToastContext.Provider>;
}

export function useToast() {
  const value = useContext(ToastContext);
  if (!value) throw new Error('useToast must be used inside ToastProvider');
  return value;
}
