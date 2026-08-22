import { CirclePlus, Server as ServerIcon } from 'lucide-react';
import { useEffect, useState, type FormEvent } from 'react';
import { useSearchParams } from 'react-router-dom';
import { api } from '../api';
import { ActionButton } from '../components/ui/ActionButton';
import { Modal } from '../components/ui/Modal';
import { EmptyState } from '../components/ui/States';
import { useI18n } from '../i18n';
import type { ActionResponse, Metrics, Runtime, Server } from '../types';
import { ServerCard } from './Dashboard';

export function ServersPage({ servers, catalog, busy, act, setBusy, onCreated, onError }: {
  servers: Server[]; catalog: Runtime[]; busy: boolean; act: (server: Server, action: string) => Promise<void>; setBusy: (busy: boolean) => void; onCreated: () => Promise<void>; onError: (error: unknown) => void;
}) {
  const { tr } = useI18n(); const [search, setSearch] = useSearchParams();
  const [metrics, setMetrics] = useState<Record<string, Metrics | null>>({});
  const open = search.get('create') === '1';
  useEffect(() => {
    let active = true;
    Promise.allSettled(servers.map((server) => api<Metrics>(`/api/v1/servers/${server.id}/metrics`))).then((results) => {
      if (active) setMetrics(Object.fromEntries(servers.map((server, index) => [server.id, results[index].status === 'fulfilled' ? results[index].value : null])));
    });
    return () => { active = false; };
  }, [servers]);
  const close = () => setSearch({});
  return <div className="page-stack"><div className="page-heading"><div><p className="eyebrow">INSTANCES</p><h1>{tr('Server', 'Servers')}</h1><p>{tr('Kelola seluruh instance Minecraft pada node ini.', 'Manage every Minecraft instance on this node.')}</p></div><ActionButton icon={CirclePlus} onClick={() => setSearch({ create: '1' })}>{tr('Buat Server', 'Create Server')}</ActionButton></div>
    <section className="surface server-inventory">{servers.length === 0 ? <EmptyState icon={ServerIcon} title={tr('Belum ada server', 'No servers yet')} description={tr('Buat server Minecraft pertama Anda untuk memulai.', 'Create your first Minecraft server to get started.')} action={{ label: tr('Buat Server', 'Create Server'), onClick: () => setSearch({ create: '1' }) }} /> : <div className="server-card-list inventory">{servers.map((server) => <ServerCard key={server.id} server={server} metrics={metrics[server.id]} busy={busy} act={act} />)}</div>}</section>
    <Modal open={open} onClose={close} title={tr('Buat Server Minecraft', 'Create Minecraft Server')}><CreateServerForm catalog={catalog} busy={busy} setBusy={setBusy} onCreated={async () => { await onCreated(); close(); }} onError={onError} /></Modal>
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
