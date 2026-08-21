import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';
import { api, ApiError, setSession } from './api';
import { BackupsPanel, ConsolePanel, FilesPanel, OverviewPanel, SchedulesPanel, SettingsPanel } from './panels';
import { formatDate } from './format';
import type { ActionResponse, AuditEvent, Runtime, Server, Session } from './types';

type Section = 'servers' | 'activity';
type ServerTab = 'overview' | 'console' | 'files' | 'backups' | 'schedules' | 'settings';

export function App() {
  const [session, setCurrentSession] = useState<Session | null | undefined>(undefined);
  const [servers, setServers] = useState<Server[]>([]);
  const [catalog, setCatalog] = useState<Runtime[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [section, setSection] = useState<Section>('servers');
  const [tab, setTab] = useState<ServerTab>('overview');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);

  const signOut = useCallback(() => {
    setSession(null);
    setCurrentSession(null);
    setServers([]);
  }, []);

  const handleError = useCallback((value: unknown) => {
    if (value instanceof ApiError && value.status === 401) {
      signOut();
      return;
    }
    setError(value instanceof Error ? value.message : 'Terjadi kesalahan yang tidak diketahui.');
  }, [signOut]);

  const loadServers = useCallback(async () => {
    try {
      const items = await api<Server[]>('/api/v1/servers');
      setServers(items);
      setSelectedId((current) => current && items.some((item) => item.id === current) ? current : items[0]?.id ?? null);
    } catch (value) {
      handleError(value);
    }
  }, [handleError]);

  useEffect(() => {
    api<Session>('/api/v1/auth/me')
      .then(async (me) => {
        setSession(me);
        setCurrentSession(me);
        const [runtimeItems, serverItems] = await Promise.all([
          api<Runtime[]>('/api/v1/catalog'),
          api<Server[]>('/api/v1/servers'),
        ]);
        setCatalog(runtimeItems);
        setServers(serverItems);
        setSelectedId(serverItems[0]?.id ?? null);
      })
      .catch(() => signOut());
  }, [signOut]);

  useEffect(() => {
    if (!session) return;
    const interval = window.setInterval(loadServers, servers.some((item) => item.currentJob) ? 1500 : 5000);
    return () => window.clearInterval(interval);
  }, [loadServers, servers, session]);

  const selected = useMemo(() => servers.find((item) => item.id === selectedId) ?? null, [servers, selectedId]);

  const act = async (server: Server, action: string) => {
    setBusy(true);
    setError('');
    try {
      await api<ActionResponse>(`/api/v1/servers/${server.id}/actions`, {
        method: 'POST', body: JSON.stringify({ action }),
      });
      setNotice(`${action} masuk ke antrean.`);
      await loadServers();
    } catch (value) {
      handleError(value);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (server: Server, purgeData: boolean) => {
    if (!window.confirm(`Hapus ${server.name}${purgeData ? ' beserta seluruh world dan file' : ' tetapi pertahankan datanya'}?`)) return;
    setBusy(true);
    setError('');
    try {
      await api<ActionResponse>(`/api/v1/servers/${server.id}`, {
        method: 'DELETE', body: JSON.stringify({ purgeData }),
      });
      setNotice('Penghapusan masuk ke antrean.');
      await loadServers();
    } catch (value) {
      handleError(value);
    } finally {
      setBusy(false);
    }
  };

  if (session === undefined) return <LoadingScreen />;
  if (!session) return <Login onLogin={(me) => { setSession(me); setCurrentSession(me); window.location.reload(); }} />;

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <button className="brand brand-button" onClick={() => { setSection('servers'); setSelectedId(null); }}>
          MY<span>PANEL</span><small>CONTROL PLANE</small>
        </button>
        <nav aria-label="Navigasi utama">
          <button className={section === 'servers' ? 'nav-active' : ''} onClick={() => setSection('servers')}>Server</button>
          <button className={section === 'activity' ? 'nav-active' : ''} onClick={() => setSection('activity')}>Aktivitas</button>
        </nav>
        <div className="sidebar-foot">
          <span className="status-dot" /> Node lokal
          <small>{session.username} · {session.role}</small>
          <button className="ghost" onClick={async () => { try { await api('/api/v1/auth/logout', { method: 'POST' }); } finally { signOut(); } }}>Keluar</button>
        </div>
      </aside>
      <main className="content">
        <header className="topbar">
          <div><p className="eyebrow">MINECRAFT JAVA · PRIVATE NODE</p><h1>{section === 'activity' ? 'Aktivitas' : selected ? selected.name : 'Server'}</h1></div>
          <div className="capacity"><b>{servers.filter((item) => item.state === 'running').length}</b><span>aktif dari {servers.length}</span></div>
        </header>
        {(error || notice) && <div className={error ? 'banner error' : 'banner success'} role={error ? 'alert' : 'status'}>
          <span>{error || notice}</span><button aria-label="Tutup pesan" onClick={() => { setError(''); setNotice(''); }}>×</button>
        </div>}
        {section === 'activity' ? <Activity onError={handleError} /> : selected ? (
          <ServerWorkspace server={selected} tab={tab} setTab={setTab} busy={busy} act={act} remove={remove}
            reload={loadServers} notify={setNotice} onError={handleError} csrfToken={session.csrfToken}
            back={() => setSelectedId(null)} />
        ) : (
          <Dashboard servers={servers} catalog={catalog} busy={busy} select={(id) => { setSelectedId(id); setTab('overview'); }}
            onCreated={async () => { setNotice('Server dibuat dan provisioning dimulai.'); await loadServers(); }} onError={handleError} setBusy={setBusy} />
        )}
      </main>
    </div>
  );
}

function LoadingScreen() {
  return <main className="center-screen"><div className="loader" aria-label="Memuat" /><p>Menghubungkan ke control plane…</p></main>;
}

export function Login({ onLogin }: { onLogin: (session: Session) => void }) {
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setBusy(true); setError('');
    const data = new FormData(event.currentTarget);
    try {
      const session = await api<Session>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ username: data.get('username'), password: data.get('password') }) });
      onLogin(session);
    } catch (value) {
      setError(value instanceof Error ? value.message : 'Login gagal.');
    } finally { setBusy(false); }
  };
  return <main className="login-screen"><section className="login-copy"><span className="brand">MY<span>PANEL</span></span><p className="eyebrow">YOUR WORLDS. YOUR INFRASTRUCTURE.</p><h1>Control plane Minecraft yang Anda miliki sendiri.</h1><p>Kelola runtime, resource, file, backup, dan console dari satu node privat.</p></section><form className="panel login-card" onSubmit={submit}><h2>Masuk ke panel</h2><p>Gunakan akun owner yang dibuat saat instalasi.</p>{error && <div className="banner error" role="alert">{error}</div>}<label>Username<input name="username" autoComplete="username" required autoFocus /></label><label>Password<input name="password" type="password" autoComplete="current-password" required /></label><button disabled={busy}>{busy ? 'Memverifikasi…' : 'Masuk'}</button></form></main>;
}

export function Dashboard({ servers, catalog, busy, select, onCreated, onError, setBusy }: {
  servers: Server[]; catalog: Runtime[]; busy: boolean; select: (id: string) => void;
  onCreated: () => Promise<void>; onError: (value: unknown) => void; setBusy: (busy: boolean) => void;
}) {
  const create = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); setBusy(true);
    const form = event.currentTarget; const data = new FormData(form);
    try {
      await api<ActionResponse>('/api/v1/servers', { method: 'POST', body: JSON.stringify({ name: data.get('name'), runtime: data.get('runtime'), version: data.get('version'), javaVersion: Number(data.get('javaVersion')), memoryMb: Number(data.get('memory')), cpu: Number(data.get('cpu')), diskMb: Number(data.get('disk')) }) });
      form.reset(); await onCreated();
    } catch (value) { onError(value); } finally { setBusy(false); }
  };
  return <div className="dashboard-grid"><section className="panel server-list"><div className="section-head"><div><p className="eyebrow">INSTANCES</p><h2>Server Anda</h2></div></div>{servers.length === 0 ? <div className="empty"><b>Belum ada server</b><span>Buat instance pertama menggunakan formulir di samping.</span></div> : <div className="server-cards">{servers.map((server) => <button key={server.id} className="server-card" onClick={() => select(server.id)}><span className={`state-icon ${server.state}`} aria-hidden="true" /><span><b>{server.name}</b><small>{server.runtime} · {server.version} · Java {server.javaVersion} · {server.bindIp}:{server.port}</small></span><span className={`badge ${server.state}`}>{server.currentJob ? `${server.currentJob.action}…` : server.state}</span></button>)}</div>}</section><form className="panel create-form" onSubmit={create}><p className="eyebrow">NEW INSTANCE</p><h2>Buat server</h2><label>Nama<input name="name" required maxLength={48} placeholder="Survival SMP" /></label><div className="form-row"><label>Runtime<select name="runtime">{catalog.map((runtime) => <option key={runtime.id} value={runtime.id}>{runtime.name}</option>)}</select></label><label>Java<select name="javaVersion" defaultValue="21"><option value="21">Java 21 (kompatibel)</option><option value="25">Java 25 (terbaru)</option></select></label></div><small className="form-help">Java 25 cocok untuk server modern. Mod atau Forge lama mungkin tetap memerlukan Java 21.</small><div className="form-row"><label>Versi<input name="version" defaultValue="1.21.4" required /></label><label>RAM<input name="memory" type="number" min="1024" max="8192" defaultValue="2048" required /><small>MB total</small></label></div><div className="form-row"><label>vCPU<input name="cpu" type="number" min="1" max="7" defaultValue="2" required /></label><label>Disk<input name="disk" type="number" min="1024" max="102400" defaultValue="10240" required /><small>MB</small></label></div><button disabled={busy}>{busy ? 'Membuat…' : 'Buat & provision'}</button></form></div>;
}

function ServerWorkspace({ server, tab, setTab, busy, act, remove, reload, notify, onError, csrfToken, back }: {
  server: Server; tab: ServerTab; setTab: (tab: ServerTab) => void; busy: boolean;
  act: (server: Server, action: string) => Promise<void>; remove: (server: Server, purge: boolean) => Promise<void>;
  reload: () => Promise<void>; notify: (message: string) => void; onError: (value: unknown) => void; csrfToken: string; back: () => void;
}) {
  const tabs: Array<[ServerTab, string]> = [['overview', 'Ringkasan'], ['console', 'Console'], ['files', 'Files'], ['backups', 'Backup'], ['schedules', 'Jadwal'], ['settings', 'Pengaturan']];
  return <><div className="workspace-actions"><button className="ghost back" onClick={back}>← Semua server</button><div><button disabled={busy || !!server.currentJob || server.state === 'running'} onClick={() => act(server, 'start')}>Start</button><button className="warning" disabled={busy || !!server.currentJob || server.state !== 'running'} onClick={() => act(server, 'restart')}>Restart</button><button className="danger" disabled={busy || !!server.currentJob || server.state !== 'running'} onClick={() => act(server, 'stop')}>Stop</button></div></div>{server.currentJob && <div className="job-strip"><span className="loader small" /><b>{server.currentJob.action}</b><span>Job sedang {server.currentJob.status}. Operasi tetap berjalan bila halaman ditutup.</span></div>}{server.lastError && <div className="banner error">{server.lastError}</div>}<nav className="tabs" aria-label="Fitur server">{tabs.map(([value, label]) => <button key={value} className={tab === value ? 'active' : ''} onClick={() => setTab(value)}>{label}</button>)}</nav>{tab === 'overview' && <OverviewPanel server={server} />}{tab === 'console' && <ConsolePanel server={server} csrfToken={csrfToken} />}{tab === 'files' && <FilesPanel server={server} notify={notify} onError={onError} />}{tab === 'backups' && <BackupsPanel server={server} reloadServer={reload} notify={notify} onError={onError} />}{tab === 'schedules' && <SchedulesPanel server={server} notify={notify} onError={onError} />}{tab === 'settings' && <SettingsPanel server={server} reload={reload} notify={notify} onError={onError} remove={remove} />}</>;
}

function Activity({ onError }: { onError: (value: unknown) => void }) {
  const [items, setItems] = useState<AuditEvent[] | null>(null);
  useEffect(() => { api<AuditEvent[]>('/api/v1/audit').then(setItems).catch(onError); }, [onError]);
  return <section className="panel"><div className="section-head"><div><p className="eyebrow">AUDIT LOG</p><h2>100 aktivitas terakhir</h2></div></div>{items === null ? <p className="muted">Memuat aktivitas…</p> : items.length === 0 ? <div className="empty">Belum ada aktivitas.</div> : <div className="audit-list">{items.map((item) => <article key={item.id}><span className="audit-icon" /><div><b>{item.action}</b><small>{item.username ?? 'system'} · {formatDate(item.createdAt)} · {item.ip ?? 'internal'}</small></div></article>)}</div>}</section>;
}
