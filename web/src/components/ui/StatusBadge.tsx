import type { ServerState } from '../../types';
import { useI18n } from '../../i18n';

export type DisplayStatus = 'running' | 'stopped' | 'starting' | 'stopping' | 'error';

export function normalizeStatus(status: ServerState): DisplayStatus {
  if (status === 'offline') return 'stopped';
  if (status === 'installing') return 'starting';
  if (status === 'deleting') return 'stopping';
  return status;
}

export function StatusBadge({ status, compact = false, label }: { status: DisplayStatus; compact?: boolean; label?: string }) {
  const { tr } = useI18n();
  const labels: Record<DisplayStatus, string> = {
    running: tr('Berjalan', 'Running'), stopped: tr('Berhenti', 'Stopped'), starting: tr('Memulai', 'Starting'),
    stopping: tr('Menghentikan', 'Stopping'), error: tr('Error', 'Error'),
  };
  return <span className={`status-badge ${status} ${compact ? 'compact' : ''}`} title={label ?? labels[status]}>
    <span className="status-indicator" aria-hidden="true" />
    {!compact && <span>{label ?? labels[status]}</span>}
    {compact && <span className="sr-only">{label ?? labels[status]}</span>}
  </span>;
}
