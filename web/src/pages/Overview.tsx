import { CalendarDays, Cpu, Globe2, HardDrive, MemoryStick, Users } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../api';
import { MetricBar } from '../components/ui/MetricBar';
import { Skeleton } from '../components/ui/States';
import { StatusBadge, normalizeStatus } from '../components/ui/StatusBadge';
import { useI18n } from '../i18n';
import { formatBytes, formatDate } from '../format';
import type { Metrics, Server } from '../types';

export function Overview({ server }: { server: Server }) {
  const { tr } = useI18n(); const [metrics, setMetrics] = useState<Metrics | null>(null); const [unavailable, setUnavailable] = useState(false);
  useEffect(() => { let active = true; const load = () => api<Metrics>(`/api/v1/servers/${server.id}/metrics`).then((value) => { if (active) { setMetrics(value); setUnavailable(false); } }).catch(() => { if (active) setUnavailable(true); }); load(); const timer = window.setInterval(load, 3000); return () => { active = false; window.clearInterval(timer); }; }, [server.id]);
  return <div className="page-stack"><div className="page-heading server-title"><div><p className="eyebrow">INSTANCE OVERVIEW</p><h1>{server.name}</h1><p>{server.runtime} {server.version} · Java {server.javaVersion}</p></div><StatusBadge status={normalizeStatus(server.state)} /></div>
    <div className="overview-grid"><section className="surface"><div className="section-heading"><div><h2>{tr('Identitas Server', 'Server Identity')}</h2><p>{tr('Detail provisioning dan jaringan', 'Provisioning and network details')}</p></div></div><div className="detail-list"><div><Globe2 /><span><small>{tr('Alamat', 'Allocation')}</small><b>{server.bindIp}:{server.port}</b></span></div><div><Cpu /><span><small>Runtime</small><b>{server.runtime} · Java {server.javaVersion}</b></span></div><div><CalendarDays /><span><small>{tr('Dibuat', 'Created')}</small><b>{formatDate(server.createdAt)}</b></span></div></div></section>
      <section className="surface"><div className="section-heading"><div><h2>{tr('Metric Langsung', 'Live Metrics')}</h2><p>{tr('Diperbarui setiap 3 detik', 'Updated every 3 seconds')}</p></div>{unavailable && <span className="unavailable">offline</span>}</div>{!metrics && !unavailable ? <Skeleton lines={4} /> : <><MetricBar label="CPU" value={metrics?.cpuPercent ?? 0} max={server.cpu * 100} unit="%" detail={`${server.cpu} vCPU limit`} /><MetricBar label="RAM" value={metrics?.memoryBytes ?? 0} max={server.memoryMb * 1024 * 1024} detail={`${formatBytes(metrics?.memoryBytes ?? 0)} / ${formatBytes(server.memoryMb * 1024 * 1024)}`} /><MetricBar label="Disk" value={metrics?.diskBytes ?? 0} max={server.diskMb * 1024 * 1024} detail={`${formatBytes(metrics?.diskBytes ?? 0)} / ${formatBytes(server.diskMb * 1024 * 1024)}`} /><div className="players-stat"><Users /><span><b>{metrics?.players ?? 0}</b><small>{tr('pemain terdeteksi', 'players detected')}</small></span></div></>}</section>
    </div>
    {server.currentJob && <div className="job-strip"><span className="spinner" /><div><b>{server.currentJob.action}</b><small>{tr(`Job sedang ${server.currentJob.status}. Operasi tetap berjalan saat halaman ditutup.`, `Job is ${server.currentJob.status}. It continues if this page is closed.`)}</small></div></div>}
    {server.lastError && <div className="inline-error" role="alert">{server.lastError}</div>}
  </div>;
}
