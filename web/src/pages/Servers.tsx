import { CirclePlus, Server as ServerIcon } from 'lucide-react';
import { useEffect, useState, type FormEvent } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../api';
import { ActionButton } from '../components/ui/ActionButton';
import { Modal } from '../components/ui/Modal';
import { EmptyState } from '../components/ui/States';
import { useI18n } from '../i18n';
import type { ActionResponse, Metrics, PanelUser, Runtime, Server } from '../types';
import { ServerCard } from './Dashboard';

export function ServersPage({ servers, catalog, busy, act, setBusy, onCreated, onError, canCreate = true }: {
  servers: Server[]; catalog: Runtime[]; busy: boolean; act: (server: Server, action: string) => Promise<void>; setBusy: (busy: boolean) => void; onCreated: () => Promise<void>; onError: (error: unknown) => void; canCreate?: boolean;
}) {
  const { tr } = useI18n(); const [search, setSearch] = useSearchParams(); const navigate = useNavigate();
  const [metrics, setMetrics] = useState<Record<string, Metrics | null>>({});
  const [query, setQuery] = useState(''); const [transfer, setTransfer] = useState<Server | null>(null); const [users, setUsers] = useState<PanelUser[]>([]); const [transferBusy, setTransferBusy] = useState(false);
  const open = search.get('create') === '1';
  useEffect(() => {
    let active = true;
    Promise.allSettled(servers.map((server) => api<Metrics>(`/api/v1/servers/${server.id}/metrics`))).then((results) => {
      if (active) setMetrics(Object.fromEntries(servers.map((server, index) => [server.id, results[index].status === 'fulfilled' ? results[index].value : null])));
    });
    return () => { active = false; };
  }, [servers]);
  useEffect(() => { if (canCreate) api<PanelUser[]>('/api/v1/admin/users').then((items) => setUsers(items.filter((item) => item.role === 'user' && item.status === 'active'))).catch(onError); }, [canCreate, onError]);
  const close = () => setSearch({});
  const visible = servers.filter((server) => `${server.name} ${server.id} ${server.ownerUsername}`.toLowerCase().includes(query.toLowerCase()));
  const transferOwner = async (event: FormEvent<HTMLFormElement>) => { event.preventDefault(); if (!transfer) return; setTransferBusy(true); const data = new FormData(event.currentTarget); try { await api(`/api/v1/admin/servers/${transfer.id}/owner`, { method: 'PUT', body: JSON.stringify({ userId: data.get('userId') }) }); setTransfer(null); await onCreated(); } catch (error) { onError(error); } finally { setTransferBusy(false); } };
  return <div className="page-stack"><div className="page-heading"><div><p className="eyebrow">INSTANCES</p><h1>{tr('Server', 'Servers')}</h1><p>{tr('Kelola instance Minecraft yang dapat Anda akses.', 'Manage Minecraft instances you can access.')}</p></div>{canCreate && <ActionButton icon={CirclePlus} onClick={() => setSearch({ create: '1' })}>{tr('Buat Server', 'Create Server')}</ActionButton>}</div>
    <section className="surface server-inventory">{servers.length === 0 ? <EmptyState icon={ServerIcon} title={tr('Belum ada server', 'No servers yet')} description={canCreate ? tr('Buat server Minecraft pertama Anda untuk memulai.', 'Create your first Minecraft server to get started.') : tr('Pilih paket hosting untuk membuat server pertama Anda.', 'Choose a hosting package to create your first server.')} action={{ label: canCreate ? tr('Buat Server', 'Create Server') : tr('Beli Server', 'Buy Server'), onClick: () => canCreate ? setSearch({ create: '1' }) : navigate('/marketplace') }} /> : <><div className="inventory-toolbar"><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder={tr('Cari nama, ID, atau pemilik…', 'Search name, ID, or owner…')} aria-label={tr('Cari server', 'Search servers')} /></div>{visible.length === 0 ? <EmptyState icon={ServerIcon} title={tr('Server tidak ditemukan', 'No matching servers')} description={tr('Coba kata kunci nama, ID, atau pemilik yang lain.', 'Try another name, ID, or owner keyword.')} /> : <div className="server-card-list inventory">{visible.map((server) => <ServerCard key={server.id} server={server} metrics={metrics[server.id]} busy={busy} act={act} showOwner={canCreate} onTransfer={canCreate ? setTransfer : undefined} />)}</div>}</>}</section>
    <Modal open={open && canCreate} onClose={close} title={tr('Buat Server Minecraft', 'Create Minecraft Server')}><CreateServerForm catalog={catalog} busy={busy} setBusy={setBusy} onCreated={async () => { await onCreated(); close(); }} onError={onError} /></Modal>
    <Modal open={!!transfer} onClose={() => setTransfer(null)} title={tr(`Alihkan ${transfer?.name ?? ''}`, `Transfer ${transfer?.name ?? ''}`)}><form className="form-stack" onSubmit={transferOwner}><label>{tr('Pemilik baru', 'New owner')}<select name="userId" required defaultValue=""><option value="" disabled>{tr('Pilih user aktif', 'Select an active user')}</option>{users.map((user) => <option key={user.id} value={user.id}>{user.username} · {user.id}</option>)}</select></label><p className="form-note">{tr('Server dan langganannya akan dipindahkan ke akun yang dipilih.', 'The server and its subscription will move to the selected account.')}</p><ActionButton type="submit" loading={transferBusy}>{tr('Alihkan server', 'Transfer server')}</ActionButton></form></Modal>
  </div>;
}

export function CreateServerForm({ catalog, busy, setBusy, onCreated, onError }: { catalog: Runtime[]; busy: boolean; setBusy: (busy: boolean) => void; onCreated: () => Promise<void>; onError: (error: unknown) => void }) {
  const { tr } = useI18n();
  const [runtimeId, setRuntimeId] = useState(catalog[0]?.id ?? 'paper');
  const runtime = catalog.find((item) => item.id === runtimeId);
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setBusy(true); const form = event.currentTarget; const data = new FormData(form);
    try {
      await api<ActionResponse>('/api/v1/servers', { method: 'POST', body: JSON.stringify({ name: data.get('name'), runtime: data.get('runtime'), version: data.get('version'), javaVersion: Number(data.get('javaVersion')), memoryMb: Number(data.get('memory')), cpu: Number(data.get('cpu')), diskMb: Number(data.get('disk')) }) });
      form.reset(); await onCreated();
    } catch (error) { onError(error); } finally { setBusy(false); }
  };
  return <form className="form-stack" onSubmit={submit}>
    <label>{tr('Nama server', 'Server name')}<input name="name" required maxLength={48} placeholder="BocahSMP" /></label>
    <div className="form-row"><label>Runtime<select name="runtime" value={runtimeId} onChange={(event) => setRuntimeId(event.target.value)}>{catalog.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label><label>Java<select key={runtimeId} name="javaVersion" defaultValue={String(runtime?.java ?? 21)}>{(runtime?.javaVersions ?? [21]).map((version) => <option key={version} value={version}>Java {version}</option>)}</select></label></div>
    <label>{tr('Versi Minecraft', 'Minecraft version')}<input name="version" defaultValue="1.21.4" required /></label>
    <div className="form-row"><label>RAM<input name="memory" type="number" min="1024" max="8192" defaultValue="2048" required /><small>MiB</small></label><label>vCPU<input name="cpu" type="number" min="1" max="7" defaultValue="2" required /></label></div>
    <label>Disk<input name="disk" type="number" min="1024" max="102400" defaultValue="10240" required /><small>MiB</small></label>
    <p className="form-note">{tr('Java 21 memberi kompatibilitas luas. Gunakan Java 25 hanya untuk runtime dan plugin yang mendukungnya.', 'Java 21 offers broad compatibility. Use Java 25 only with runtimes and plugins that support it.')}</p>
    <ActionButton type="submit" loading={busy}>{tr('Buat dan provision', 'Create and provision')}</ActionButton>
  </form>;
}
