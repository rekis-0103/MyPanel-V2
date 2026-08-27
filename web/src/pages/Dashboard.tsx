import { Activity, ArrowRight, CirclePlus, Cpu, HardDrive, MemoryStick, Server as ServerIcon, Terminal, Users } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api';
import { ActionButton } from '../components/ui/ActionButton';
import { EmptyState, Skeleton } from '../components/ui/States';
import { MetricBar } from '../components/ui/MetricBar';
import { StatusBadge, normalizeStatus } from '../components/ui/StatusBadge';
import { useI18n } from '../i18n';
import { formatBytes, formatDate, serverAddress } from '../format';
import type { AuditEvent, Metrics, Server } from '../types';

export function Dashboard({ servers, busy, act, onError, isAdmin = true }: { servers: Server[]; busy: boolean; act: (server: Server, action: string) => Promise<void>; onError: (error: unknown) => void; isAdmin?: boolean }) {
  const { tr } = useI18n(); const navigate = useNavigate();
  const [metrics, setMetrics] = useState<Record<string, Metrics | null>>({});
  const [activity, setActivity] = useState<AuditEvent[] | null>(null);
  const load = useCallback(async () => {
    const results = await Promise.allSettled(servers.map((server) => api<Metrics>(`/api/v1/servers/${server.id}/metrics`)));
    setMetrics(Object.fromEntries(servers.map((server, index) => [server.id, results[index].status === 'fulfilled' ? results[index].value : null])));
  }, [servers]);
  useEffect(() => { load(); const timer = window.setInterval(load, 5000); return () => window.clearInterval(timer); }, [load]);
  useEffect(() => { api<AuditEvent[]>('/api/v1/audit').then((items) => setActivity(items.slice(0, 6))).catch(onError); }, [onError]);
  const allocated = servers.reduce((sum, server) => ({ cpu: sum.cpu + server.cpu, ram: sum.ram + server.memoryMb, disk: sum.disk + server.diskMb }), { cpu: 0, ram: 0, disk: 0 });
  const createPath=isAdmin?'/servers?create=1':'/marketplace';
  return <div className="page-stack">
    <div className="page-heading"><div><p className="eyebrow">CONTROL PLANE</p><h1>{tr('Selamat datang kembali', 'Welcome back')}</h1><p>{isAdmin?tr('Pantau server dan aktivitas node dari satu tempat.', 'Monitor servers and node activity from one place.'):tr('Kelola server dan masa aktif layanan Anda.', 'Manage your servers and service periods.')}</p></div><ActionButton icon={CirclePlus} onClick={() => navigate(createPath)}>{isAdmin?tr('Buat Server', 'Create Server'):tr('Beli Server', 'Buy Server')}</ActionButton></div>
    <div className="dashboard-grid">
      <section className="surface server-list-panel"><div className="section-heading"><div><h2>{tr('Server Minecraft', 'Minecraft Servers')}</h2><p>{tr(`${servers.filter((server) => server.state === 'running').length} dari ${servers.length} berjalan`, `${servers.filter((server) => server.state === 'running').length} of ${servers.length} running`)}</p></div><button className="text-button" onClick={() => navigate('/servers')}>{tr('Lihat semua', 'View all')}<ArrowRight /></button></div>
        {servers.length === 0 ? <EmptyState icon={ServerIcon} title={tr('Belum ada server', 'No servers yet')} description={tr('Buat server Minecraft pertama Anda untuk memulai.', 'Create your first Minecraft server to get started.')} action={{ label: isAdmin?tr('Buat Server', 'Create Server'):tr('Beli Server','Buy Server'), onClick: () => navigate(createPath) }} /> : <div className="server-card-list">{servers.map((server) => <ServerCard key={server.id} server={server} metrics={metrics[server.id]} busy={busy} act={act} showOwner={isAdmin} />)}</div>}
      </section>
      {isAdmin && <section className="surface node-overview"><div className="section-heading"><div><h2>{tr('Ringkasan Node', 'Node Overview')}</h2><p>local</p></div><span className="unavailable">{tr('Buka menu Kapasitas', 'Open Capacity')}</span></div>
        <div className="allocation-summary"><div><Cpu /><span><b>{allocated.cpu} vCPU</b><small>{tr('dialokasikan', 'allocated')}</small></span></div><div><MemoryStick /><span><b>{formatBytes(allocated.ram * 1024 * 1024)}</b><small>RAM {tr('dialokasikan', 'allocated')}</small></span></div><div><HardDrive /><span><b>{formatBytes(allocated.disk * 1024 * 1024)}</b><small>disk {tr('dialokasikan', 'allocated')}</small></span></div></div>
        <MetricBar label="CPU host" value={0} max={100} unit="%" detail={tr('Hubungkan endpoint telemetri node', 'Connect the node telemetry endpoint')} />
        <MetricBar label="RAM host" value={0} max={100} unit="%" detail={tr('Kapasitas host belum dilaporkan', 'Host capacity is not reported')} />
        <MetricBar label="Disk host" value={0} max={100} unit="%" detail={tr('Kapasitas host belum dilaporkan', 'Host capacity is not reported')} />
        <div className="uptime"><span>{tr('Uptime node', 'Node uptime')}</span><b>—</b></div>
      </section>}
    </div>
    <section className="surface activity-panel"><div className="section-heading"><div><h2>{tr('Aktivitas Terbaru', 'Recent Activity')}</h2><p>{tr('Perubahan terbaru pada panel', 'Latest control-plane changes')}</p></div><button className="text-button" onClick={() => navigate('/activity')}>{tr('Buka log', 'Open log')}<ArrowRight /></button></div>
      {activity === null ? <Skeleton lines={4} /> : activity.length === 0 ? <EmptyState icon={Activity} title={tr('Belum ada aktivitas', 'No activity yet')} description={tr('Mulai atau konfigurasi server untuk menghasilkan catatan aktivitas.', 'Start or configure a server to create activity records.')} action={{ label: tr('Kelola Server', 'Manage Servers'), onClick: () => navigate('/servers') }} /> : <div className="activity-list">{activity.map((item) => <article key={item.id}><span className="activity-mark" /><div><b>{item.action}</b><small>{item.username ?? 'system'} · {formatDate(item.createdAt)}</small></div></article>)}</div>}
    </section>
  </div>;
}

export function ServerCard({ server, metrics, busy, act, showOwner = false, onTransfer }: { server: Server; metrics: Metrics | null | undefined; busy: boolean; act: (server: Server, action: string) => Promise<void>; showOwner?: boolean; onTransfer?: (server: Server) => void }) {
  const { tr } = useI18n(); const navigate = useNavigate(); const running = server.state === 'running';
  const inactive=server.subscriptionStatus==='grace'||server.subscriptionStatus==='released'||server.subscriptionStatus==='canceled';
  return <article className="server-card" onClick={() => navigate(inactive?'/billing':`/servers/${server.id}/console`)}>
    <div className="server-card-head"><StatusBadge status={normalizeStatus(server.state)} compact /><div><h3>{server.name}</h3><p>{server.runtime} {server.version} · Java {server.javaVersion} · {serverAddress(server.bindIp, server.port)}{showOwner && ` · ${tr('pemilik', 'owner')}: ${server.ownerUsername}`}</p></div></div>
    <div className="server-inline-metrics"><span><Users />{metrics?.players ?? 0}/{Number(server.config?.maxPlayers ?? 20)}</span><span><MemoryStick />{metrics ? formatBytes(metrics.memoryBytes) : '—'} / {formatBytes(server.memoryMb * 1024 * 1024)}</span><span><Cpu />{metrics ? `${metrics.cpuPercent.toFixed(1)}%` : '—'}</span></div>
    <div className="server-quick-actions" onClick={(event) => event.stopPropagation()}>{inactive ? <ActionButton size="sm" variant="secondary" onClick={() => navigate('/billing')}>{tr('Perpanjang', 'Renew')}</ActionButton> : <>
      <ActionButton size="sm" variant={running ? 'danger' : 'primary'} loading={busy || !!server.currentJob} disabled={server.state === 'starting' || server.state === 'stopping'} onClick={() => act(server, running ? 'stop' : 'start')}>{running ? tr('Stop', 'Stop') : tr('Mulai', 'Start')}</ActionButton>
      <ActionButton size="sm" variant="ghost" icon={Terminal} aria-label={tr(`Buka console ${server.name}`, `Open ${server.name} console`)} title={tr('Buka console', 'Open console')} onClick={() => navigate(`/servers/${server.id}/console`)} />{onTransfer && <ActionButton size="sm" variant="secondary" onClick={() => onTransfer(server)}>{tr('Alihkan', 'Transfer')}</ActionButton>}</>}
    </div>
  </article>;
}
