import { Component, type ErrorInfo, type ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { ActionButton } from './ActionButton';

export function EmptyState({ icon: Icon, title, description, action }: { icon: LucideIcon; title: string; description: string; action?: { label: string; onClick: () => void } }) {
  return <div className="empty-state"><Icon aria-hidden="true" /><h3>{title}</h3><p>{description}</p>{action && <ActionButton size="sm" onClick={action.onClick}>{action.label}</ActionButton>}</div>;
}

export function Skeleton({ lines = 3 }: { lines?: number }) {
  return <div className="skeleton" aria-label="Loading">{Array.from({ length: lines }, (_, index) => <span key={index} />)}</div>;
}

export class PageErrorBoundary extends Component<{ children: ReactNode; message: string; reloadLabel?: string }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  componentDidCatch(error: Error, info: ErrorInfo) { console.error('Page render failed', error, info.componentStack); }
  render() {
    if (this.state.failed) return <div className="page-error" role="alert"><AlertCircleIcon /><h2>{this.props.message}</h2><button onClick={() => window.location.reload()}>{this.props.reloadLabel ?? 'Reload'}</button></div>;
    return this.props.children;
  }
}

function AlertCircleIcon() { return <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 7v6m0 4h.01"/></svg>; }
