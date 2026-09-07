import { CirclePlus, Server as ServerIcon } from 'lucide-react';
import { useEffect, useState, type FormEvent } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../api';
import { ActionButton } from '../components/ui/ActionButton';
import { Modal } from '../components/ui/Modal';
import { EmptyState } from '../components/ui/States';
import { useI18n } from '../i18n';
import { useServerMetrics } from '../useServerMetrics';
import type { ActionResponse, Capacity, PanelUser, Runtime, Server } from '../types';
import { ServerCard } from './Dashboard';

export function ServersPage({ servers, catalog, capacity, pendingActions, act, onCreated, onError, canCreate = true }: {
  servers: Server[]; catalog: Runtime[]; capacity: Capacity | null; pendingActions: Record<string, string>; act: (server: Server, action: string) => Promise<void>; onCreated: () => Promise<void>; onError: (error: unknown) => void; canCreate?: boolean;
}) {
  const { tr } = useI18n(); const [search, setSearch] = useSearchParams(); const navigate = useNavigate();
  const metrics = useServerMetrics(servers.map((server) => server.id));
  const [query, setQuery] = useState(''); const [transfer, setTransfer] = useState<Server | null>(null); const [users, setUsers] = useState<PanelUser[]>([]); const [transferBusy, setTransferBusy] = useState(false);
  const open = search.get('create') === '1';
  useEffect(() => { if (canCreate) api<PanelUser[]>('/api/v1/admin/users').then((items) => setUsers(items.filter((item) => item.role === 'user' && item.status === 'active'))).catch(onError); }, [canCreate, onError]);
  const close = () => setSearch({});
  const visible = servers.filter((server) => `${server.name} ${server.id} ${server.ownerUsername}`.toLowerCase().includes(query.toLowerCase()));
  const transferOwner = async (event: FormEvent<HTMLFormElement>) => { event.preventDefault(); if (!transfer) return; setTransferBusy(true); const data = new FormData(event.currentTarget); try { await api(`/api/v1/admin/servers/${transfer.id}/owner`, { method: 'PUT', body: JSON.stringify({ userId: data.get('userId') }) }); setTransfer(null); await onCreated(); } catch (error) { onError(error); } finally { setTransferBusy(false); } };
  return <div className="page-stack"><div className="page-heading"><div><p className="eyebrow">INSTANCES</p><h1>{tr('Server', 'Servers')}</h1><p>{tr('Kelola instance Minecraft yang dapat Anda akses.', 'Manage Minecraft instances you can access.')}</p></div>{canCreate && <ActionButton icon={CirclePlus} onClick={() => setSearch({ create: '1' })}>{tr('Buat Server', 'Create Server')}</ActionButton>}</div>
    <section className="surface server-inventory">{servers.length === 0 ? <EmptyState icon={ServerIcon} title={tr('Belum ada server', 'No servers yet')} description={canCreate ? tr('Buat server Minecraft pertama Anda untuk memulai.', 'Create your first Minecraft server to get started.') : tr('Pilih paket hosting untuk membuat server pertama Anda.', 'Choose a hosting package to create your first server.')} action={{ label: canCreate ? tr('Buat Server', 'Create Server') : tr('Beli Server', 'Buy Server'), onClick: () => canCreate ? setSearch({ create: '1' }) : navigate('/marketplace') }} /> : <><div className="inventory-toolbar"><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={tr('Cari nama, ID, atau pemilik…', 'Search name, ID, or owner…')} aria-label={tr('Cari server', 'Search servers')} /></div>{visible.length === 0 ? <EmptyState icon={ServerIcon} title={tr('Server tidak ditemukan', 'No matching servers')} description={tr('Coba kata kunci nama, ID, atau pemilik yang lain.', 'Try another name, ID, or owner keyword.')} /> : <div className="server-card-list inventory">{visible.map((server) => <ServerCard key={server.id} server={server} metrics={metrics[server.id]} busy={!!pendingActions[server.id]} act={act} showOwner={canCreate} onTransfer={canCreate ? setTransfer : undefined} />)}</div>}</>}</section>
    <Modal open={open && canCreate} onClose={close} title={tr('Buat Server Minecraft', 'Create Minecraft Server')}><CreateServerForm catalog={catalog} capacity={capacity} onCreated={async () => { await onCreated(); close(); }} onError={onError} /></Modal>
    <Modal open={!!transfer} onClose={() => setTransfer(null)} title={tr(`Alihkan ${transfer?.name ?? ''}`, `Transfer ${transfer?.name ?? ''}`)}><form className="form-stack" onSubmit={transferOwner}><label>{tr('Pemilik baru', 'New owner')}<select name="userId" required defaultValue=""><option value="" disabled>{tr('Pilih user aktif', 'Select an active user')}</option>{users.map((user) => <option key={user.id} value={user.id}>{user.username} · {user.id}</option>)}</select></label><p className="form-note">{tr('Server dan langganannya akan dipindahkan ke akun yang dipilih.', 'The server and its subscription will move to the selected account.')}</p><ActionButton type="submit" loading={transferBusy}>{tr('Alihkan server', 'Transfer server')}</ActionButton></form></Modal>
  </div>;
}

export function CreateServerForm({ catalog, capacity, onCreated, onError }: { catalog: Runtime[]; capacity?: Capacity | null; onCreated: () => Promise<void>; onError: (error: unknown) => void }) {
  const { tr } = useI18n();
  const [busy, setBusy] = useState(false);
  const [runtimeId, setRuntimeId] = useState(catalog[0]?.id ?? 'paper');
  const runtime = catalog.find((item) => item.id === runtimeId);
  const versions = runtime?.versions ?? [];
  const [version, setVersion] = useState(versions[0]?.id ?? '1.21.4');
  const selectedVersion = versions.find((item) => item.id === version);
  const available = capacity?.available;
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setBusy(true); const form = event.currentTarget; const data = new FormData(form);
    try {
      await api<ActionResponse>('/api/v1/servers', { method: 'POST', body: JSON.stringify({ name: data.get('name'), runtime: data.get('runtime'), version: data.get('version'), javaVersion: Number(data.get('javaVersion')), memoryMb: Number(data.get('memory')), cpu: Number(data.get('cpu')), diskMb: Number(data.get('disk')) }) });
      form.reset(); await onCreated();
    } catch (error) { onError(error); } finally { setBusy(false); }
  };
  return <form className="form-stack" onSubmit={submit}>
    <label>{tr('Nama server', 'Server name')}<input name="name" required maxLength={48} placeholder="BocahSMP" /></label>
    <div className="form-row"><label>Runtime<select name="runtime" value={runtimeId} onChange={(event) => { const next = catalog.find((item) => item.id === event.target.value); setRuntimeId(event.target.value); setVersion(next?.versions?.[0]?.id ?? '1.21.4'); }}>{catalog.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label><label>Java<input name="javaVersion" value={selectedVersion?.java ?? runtime?.java ?? 21} readOnly /></label></div>
    <label>{tr('Versi Minecraft', 'Minecraft version')}{versions.length ? <select name="version" value={version} onChange={(event) => setVersion(event.target.value)}>{versions.map((item) => <option key={item.id} value={item.id}>{item.id}</option>)}</select> : <input name="version" value={version} onChange={(event) => setVersion(event.target.value)} required />}</label>
    <div className="form-row"><label>RAM<input name="memory" type="number" min="1024" max={Math.max(1024, Math.min(8192, available?.memoryMb ?? 8192))} defaultValue={Math.max(1024, Math.min(2048, available?.memoryMb ?? 2048))} required /><small>MiB</small></label><label>vCPU<input name="cpu" type="number" min="1" max={Math.max(1, available?.cpu ?? 8)} defaultValue={Math.max(1, Math.min(2, available?.cpu ?? 2))} required /></label></div>
    <label>Disk<input name="disk" type="number" min="1024" max={Math.max(1024, available?.diskMb ?? 102400)} defaultValue={Math.max(1024, Math.min(10240, available?.diskMb ?? 10240))} required /><small>MiB</small></label>
    <p className="form-note">{available ? tr(`Tersedia: ${available.cpu} vCPU, ${available.memoryMb} MiB RAM, ${available.diskMb} MiB disk. Java mengikuti versi Minecraft.`, `Available: ${available.cpu} vCPU, ${available.memoryMb} MiB RAM, ${available.diskMb} MiB disk. Java follows the Minecraft version.`) : tr('Kapasitas sedang dimuat. Nilai tetap divalidasi kembali oleh server.', 'Capacity is loading. Values are validated again by the server.')}</p>
    <ActionButton type="submit" loading={busy} disabled={!!available && (available.cpu < 1 || available.memoryMb < 1024 || available.diskMb < 1024)}>{tr('Buat dan provision', 'Create and provision')}</ActionButton>
  </form>;
}
