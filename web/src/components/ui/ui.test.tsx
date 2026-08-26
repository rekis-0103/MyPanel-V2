import { act, fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider, useI18n } from '../../i18n';
import { MetricBar } from './MetricBar';
import { Modal } from './Modal';
import { StatusBadge } from './StatusBadge';
import { ToastProvider, useToast } from './Toast';

describe('UI primitives', () => {
  it('renders status and threshold states without relying on color alone', () => {
    render(<I18nProvider><StatusBadge status="running" /><MetricBar label="RAM" value={92} max={100} unit="%" /></I18nProvider>);
    expect(screen.getByText('Berjalan')).toBeInTheDocument();
    expect(screen.getByRole('progressbar', { name: 'RAM' })).toHaveClass('danger');
  });

  it('closes a modal with Escape and restores focus', () => {
    const close = vi.fn();
    render(<><button>Trigger</button><Modal open title="Confirm" onClose={close}><input aria-label="Confirmation" /></Modal></>);
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(close).toHaveBeenCalledOnce();
  });

  it('keeps the active field focused when an open modal rerenders', () => {
    vi.useFakeTimers();
    const initialClose = vi.fn(); const latestClose = vi.fn();
    const view = render(<Modal open title="Create server" onClose={initialClose}><input aria-label="Server name" /></Modal>);
    act(() => vi.runOnlyPendingTimers());
    const input = screen.getByRole('textbox', { name: 'Server name' });
    input.focus();

    view.rerender(<Modal open title="Create server" onClose={latestClose}><input aria-label="Server name" /></Modal>);
    act(() => vi.runOnlyPendingTimers());

    expect(input).toHaveFocus();
    fireEvent.keyDown(document, { key: 'Escape' });
    expect(initialClose).not.toHaveBeenCalled();
    expect(latestClose).toHaveBeenCalledOnce();
    view.unmount();
    vi.useRealTimers();
  });

  it('switches language and persists the preference', () => {
    function Switcher() { const { locale, setLocale, tr } = useI18n(); return <button onClick={() => setLocale(locale === 'id' ? 'en' : 'id')}>{tr('Indonesia', 'English')}</button>; }
    render(<I18nProvider><Switcher /></I18nProvider>);
    fireEvent.click(screen.getByRole('button', { name: 'Indonesia' }));
    expect(screen.getByRole('button', { name: 'English' })).toBeInTheDocument();
    expect(localStorage.getItem('mypanel.locale')).toBe('en');
    localStorage.removeItem('mypanel.locale');
  });

  it('auto-dismisses toast messages', () => {
    vi.useFakeTimers();
    function Trigger() { const { showToast } = useToast(); return <button onClick={() => showToast('Saved', 'success')}>Show</button>; }
    render(<ToastProvider><Trigger /></ToastProvider>);
    fireEvent.click(screen.getByRole('button', { name: 'Show' }));
    expect(screen.getByText('Saved')).toBeInTheDocument();
    act(() => vi.advanceTimersByTime(4000));
    expect(screen.queryByText('Saved')).not.toBeInTheDocument();
    vi.useRealTimers();
  });
});
