import { Activity } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';
import { EmptyState, Skeleton } from '../components/ui/States';
import { useI18n } from '../i18n';
import { formatDate } from '../format';
import type { AuditEvent } from '../types';

export function ActivityLog({ onError }: { onError: (error: unknown) => void }) {
  const { tr } = useI18n(); const navigate = useNavigate(); const [items, setItems] = useState<AuditEvent[] | null>(null);
  useEffect(() => { api<AuditEvent[]>('/api/v1/audit').then(setItems).catch(onError); }, [onError]);
  return <div className="page-stack"><div className="page-heading"><div><p className="eyebrow">AUDIT TRAIL</p><h1>{tr('Log Aktivitas', 'Activity Log')}</h1><p>{tr('100 aksi terbaru yang tercatat oleh control plane.', 'The latest 100 actions recorded by the control plane.')}</p></div></div><section className="surface">{items === null ? <Skeleton lines={7} /> : items.length === 0 ? <EmptyState icon={Activity} title={tr('Belum ada aktivitas', 'No activity yet')} description={tr('Aktivitas server dan akun akan muncul di sini.', 'Server and account activity will appear here.')} action={{ label: tr('Kelola Server', 'Manage Servers'), onClick: () => navigate('/servers') }} /> : <div className="activity-list full">{items.map((item) => <article key={item.id}><span className="activity-mark" /><div><b>{item.action}</b><small>{item.username ?? 'system'} · {formatDate(item.createdAt)} · {item.ip ?? 'internal'}</small></div><span>{item.targetType ?? 'system'}</span></article>)}</div>}</section></div>;
}
