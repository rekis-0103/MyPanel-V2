import { LockKeyhole, Server as ServerIcon } from 'lucide-react';
import { lazy, Suspense, useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react';
import { Navigate, Route, Routes, useNavigate, useParams } from 'react-router-dom';
import { api, ApiError, setSession as setApiSession } from './api';
import { ActionButton } from './components/ui/ActionButton';
import { PageErrorBoundary } from './components/ui/States';
import { useToast } from './components/ui/Toast';
import { useI18n } from './i18n';
import { AppShell } from './layouts/AppShell';
import { ActivityLog } from './pages/ActivityLog';
import { Addons } from './pages/Addons';
import { Backups } from './pages/Backups';
import { Dashboard } from './pages/Dashboard';
import { FileManager } from './pages/FileManager';
import { Schedules } from './pages/Schedules';
import { ServerSettings } from './pages/ServerSettings';
import { ServersPage } from './pages/Servers';
import { AdminCapacity, AdminPackages, AdminUsers, Billing, Marketplace } from './pages/Hosting';
import { Landing } from './pages/Landing';
import { Modpacks } from './pages/Modpacks';
import type { ActionResponse, Capacity, Runtime, Server, Session } from './types';

const ConsolePage = lazy(() => import('./pages/Console').then((module) => ({ default: module.Console })));

export function App() {
  const { tr } = useI18n(); const { showToast } = useToast(); const navigate = useNavigate();
  const [session, setCurrentSession] = useState<Session | null | undefined>(undefined); const [servers, setServers] = useState<Server[]>([]); const [catalog, setCatalog] = useState<Runtime[]>([]); const [capacity, setCapacity] = useState<Capacity | null>(null); const [pendingActions, setPendingActions] = useState<Record<string, string>>({}); const [bootstrapFailed, setBootstrapFailed] = useState(false);
  const signOut = useCallback(() => { setApiSession(null); setCurrentSession(null); setServers([]); }, []);
  const handleError = useCallback((value: unknown) => { if (value instanceof ApiError && value.status === 401) { signOut(); return; } showToast(value instanceof Error ? value.message : tr('Terjadi kesalahan yang tidak diketahui.', 'An unknown error occurred.'), 'error'); }, [showToast, signOut, tr]);
  const loadServers = useCallback(async () => { try { setServers(await api<Server[]>('/api/v1/servers')); } catch (error) { handleError(error); } }, [handleError]);
  const loadCapacity = useCallback(async () => { try { setCapacity(await api<Capacity>('/api/v1/capacity')); } catch (error) { handleError(error); } }, [handleError]);
  const loadBootstrapData = useCallback(async () => {
    const results = await Promise.allSettled([api<Runtime[]>('/api/v1/catalog'), api<Server[]>('/api/v1/servers'), api<Capacity>('/api/v1/capacity')]);
    if (results[0].status === 'fulfilled') setCatalog(results[0].value); else handleError(results[0].reason);
    if (results[1].status === 'fulfilled') setServers(results[1].value); else handleError(results[1].reason);
    if (results[2].status === 'fulfilled') setCapacity(results[2].value); else handleError(results[2].reason);
    setBootstrapFailed(results.some((result) => result.status === 'rejected'));
  }, [handleError]);
  useEffect(() => {
    let active = true;
    api<Session>('/api/v1/auth/me').then(async (me) => {
      if (!active) return;
      setApiSession(me); setCurrentSession(me);
      await loadBootstrapData();
    }).catch((error) => {
      if (!active) return;
      if (error instanceof ApiError && error.status !== 401) { setCurrentSession(null); handleError(error); return; }
      signOut();
    });
    return () => { active = false; };
  }, [handleError, loadBootstrapData, signOut]);
  useEffect(() => { if (!session) return; const timer = window.setInterval(loadServers, servers.some((server) => server.currentJob) ? 1500 : 5000); return () => window.clearInterval(timer); }, [loadServers, servers, session]);
  useEffect(() => { if (!session || session.role !== 'owner') return; const timer = window.setInterval(loadCapacity, 15000); return () => window.clearInterval(timer); }, [loadCapacity, session]);
  const act = async (server: Server, action: string) => { setPendingActions((items) => ({ ...items, [server.id]: action })); try { const response = await api<ActionResponse>(`/api/v1/servers/${server.id}/actions`, { method: 'POST', body: JSON.stringify({ action }) }); setServers((items) => items.map((item) => item.id === server.id ? response.server : item)); showToast(tr(`${action} masuk ke antrean.`, `${action} queued.`), 'success'); } catch (error) { handleError(error); } finally { setPendingActions((items) => { const next = { ...items }; delete next[server.id]; return next; }); } };
  const remove = async (server: Server, purgeData: boolean) => { setPendingActions((items) => ({ ...items, [server.id]: 'delete' })); try { await api<ActionResponse>(`/api/v1/servers/${server.id}`, { method: 'DELETE', body: JSON.stringify({ purgeData }) }); showToast(tr('Penghapusan masuk ke antrean.', 'Deletion queued.'), 'success'); navigate('/servers'); await loadServers(); } catch (error) { handleError(error); } finally { setPendingActions((items) => { const next = { ...items }; delete next[server.id]; return next; }); } };
  const notify = useCallback((message: string, variant: 'success' | 'error' | 'info' | 'warning' = 'info') => showToast(message, variant), [showToast]);
  if (session === undefined) return <LoadingScreen />;
  if (!session) return <Routes>
    <Route index element={<Landing />} />
    <Route path="login" element={<Login onLogin={() => window.location.reload()} />} />
    <Route path="*" element={<Navigate to="/" replace />} />
  </Routes>;
  if (session.mustChangePassword) return <ChangePassword session={session} onDone={signOut} onError={handleError} />;
  if (bootstrapFailed) return <AppShell servers={servers} capacity={capacity} session={session} onLogout={async () => { try { await api('/api/v1/auth/logout', { method: 'POST' }); } finally { signOut(); } }}><div className="surface bootstrap-error"><LockKeyhole /><h1>{tr('Data panel belum lengkap', 'Panel data is incomplete')}</h1><p>{tr('Sesi Anda tetap aktif, tetapi sebagian data gagal dimuat. Periksa koneksi controller lalu coba lagi.', 'Your session remains active, but some data failed to load. Check the controller connection and retry.')}</p><ActionButton onClick={loadBootstrapData}>{tr('Coba lagi', 'Retry')}</ActionButton></div></AppShell>;
  return <AppShell servers={servers} capacity={capacity} session={session} onLogout={async () => { try { await api('/api/v1/auth/logout', { method: 'POST' }); } finally { signOut(); } }}><Routes>
    <Route index element={<Navigate to="/dashboard" replace />} />
    <Route path="dashboard" element={<Page title={tr('Dashboard gagal dimuat.', 'Dashboard failed to load.')}><Dashboard servers={servers} capacity={capacity} pendingActions={pendingActions} act={act} onError={handleError} isAdmin={session.role === 'owner'} /></Page>} />
    <Route path="servers" element={<Page title={tr('Daftar server gagal dimuat.', 'Server list failed to load.')}><ServersPage servers={servers} catalog={catalog} capacity={capacity} pendingActions={pendingActions} act={act} canCreate={session.role === 'owner'} onCreated={async () => { showToast(tr('Server dibuat dan provisioning dimulai.', 'Server created and provisioning started.'), 'success'); await loadServers(); }} onError={handleError} /></Page>} />
    {session.role === 'user' && <Route path="marketplace" element={<Page title={tr('Marketplace gagal dimuat.', 'Marketplace failed to load.')}><Marketplace catalog={catalog} reloadServers={loadServers} notify={notify} onError={handleError} /></Page>} />}
    <Route path="billing" element={<Page title={tr('Billing gagal dimuat.', 'Billing failed to load.')}><Billing notify={notify} onError={handleError} /></Page>} />
    {session.role === 'owner' && <Route path="admin/users" element={<Page title={tr('Akun gagal dimuat.', 'Accounts failed to load.')}><AdminUsers notify={notify} onError={handleError} /></Page>} />}
    {session.role === 'owner' && <Route path="admin/packages" element={<Page title={tr('Paket gagal dimuat.', 'Packages failed to load.')}><AdminPackages notify={notify} onError={handleError} /></Page>} />}
    {session.role === 'owner' && <Route path="admin/capacity" element={<Page title={tr('Kapasitas gagal dimuat.', 'Capacity failed to load.')}><AdminCapacity onError={handleError} /></Page>} />}
    <Route path="servers/:serverId" element={<ServerRedirect servers={servers} />} />
    <Route path="servers/:serverId/console" element={<ServerPage servers={servers}>{(server) => <Suspense fallback={<div className="surface"><span className="spinner" /></div>}><ConsolePage server={server} csrfToken={session.csrfToken} busy={!!pendingActions[server.id]} act={act} /></Suspense>}</ServerPage>} />
    <Route path="servers/:serverId/files" element={<ServerPage servers={servers}>{(server) => <FileManager server={server} notify={notify} onError={handleError} />}</ServerPage>} />
    <Route path="servers/:serverId/addons" element={<ServerPage servers={servers}>{(server) => <Addons server={server} notify={notify} onError={handleError} />}</ServerPage>} />
    <Route path="servers/:serverId/modpacks" element={<ServerPage servers={servers}>{(server) => <Modpacks server={server} reload={loadServers} notify={notify} onError={handleError} />}</ServerPage>} />
    <Route path="servers/:serverId/backups" element={<ServerPage servers={servers}>{(server) => <Backups server={server} notify={notify} onError={handleError} />}</ServerPage>} />
    <Route path="servers/:serverId/schedules" element={<ServerPage servers={servers}>{(server) => <Schedules server={server} notify={notify} onError={handleError} />}</ServerPage>} />
    <Route path="servers/:serverId/settings" element={<ServerPage servers={servers}>{(server) => <ServerSettings server={server} reload={loadServers} notify={notify} onError={handleError} remove={remove} />}</ServerPage>} />
    <Route path="activity" element={<Page title={tr('Log aktivitas gagal dimuat.', 'Activity log failed to load.')}><ActivityLog onError={handleError} /></Page>} />
    <Route path="*" element={<Navigate to="/dashboard" replace />} />
  </Routes></AppShell>;
}

function Page({ title, children }: { title: string; children: ReactNode }) { const { tr } = useI18n(); return <PageErrorBoundary message={title} reloadLabel={tr('Muat ulang', 'Reload')}>{children}</PageErrorBoundary>; }
function ServerRedirect({ servers }: { servers: Server[] }) { const { serverId } = useParams(); return servers.some((server) => server.id === serverId) ? <Navigate to={`/servers/${serverId}/console`} replace /> : <Navigate to="/servers" replace />; }
function ServerPage({ servers, children }: { servers: Server[]; children: (server: Server) => ReactNode }) { const { tr } = useI18n(); const { serverId } = useParams(); const server = servers.find((item) => item.id === serverId); if (!server) return <Navigate to="/servers" replace />; if (server.subscriptionStatus === 'grace' || server.subscriptionStatus === 'released' || server.subscriptionStatus === 'canceled') return <Navigate to="/billing" replace />; return <Page title={tr('Halaman server gagal dimuat.', 'Server page failed to load.')}>{children(server)}</Page>; }

function LoadingScreen() { const { tr } = useI18n(); return <main className="loading-screen"><div className="brand-emblem"><ServerIcon /></div><span className="spinner" /><p>{tr('Menghubungkan ke control plane…', 'Connecting to the control plane…')}</p></main>; }

export function Login({ onLogin }: { onLogin: (session: Session) => void }) {
  const { locale, setLocale, tr } = useI18n(); const [error, setError] = useState(''); const [busy, setBusy] = useState(false); const [registering, setRegistering] = useState(false);
  const submit = async (event: FormEvent<HTMLFormElement>) => { event.preventDefault(); setBusy(true); setError(''); const data = new FormData(event.currentTarget); try { const session = await api<Session>(registering ? '/api/v1/auth/register' : '/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ username: data.get('username'), password: data.get('password') }) }); setApiSession(session); onLogin(session); } catch (value) { setError(value instanceof Error ? value.message : tr('Login gagal.', 'Login failed.')); } finally { setBusy(false); } };
  return <main className="login-screen"><button className="login-language language-toggle" onClick={() => setLocale(locale === 'id' ? 'en' : 'id')}>{locale.toUpperCase()}</button><section className="login-intro"><div className="login-brand"><span className="brand-mark"><ServerIcon /></span><b>MyPanel <small>v2</small></b></div><p className="eyebrow">MINECRAFT HOSTING PANEL</p><h1>{tr('Server Anda. Infrastruktur Kami.', 'Your server. Our infrastructure.')}</h1><p>{tr('Beli paket simulasi dan kelola server Minecraft dari satu panel.', 'Buy a simulated package and manage Minecraft from one panel.')}</p></section><form className="surface login-card" onSubmit={submit}><LockKeyhole /><h2>{registering ? tr('Buat akun', 'Create account') : tr('Masuk ke MyPanel', 'Sign in to MyPanel')}</h2><p>{registering ? tr('Daftar menggunakan username dan password.', 'Register with a username and password.') : tr('Admin dan user masuk melalui halaman yang sama.', 'Admins and users sign in on the same page.')}</p>{error && <div className="inline-error" role="alert">{error}</div>}<label>Username<input name="username" autoComplete="username" minLength={3} maxLength={32} pattern="[A-Za-z0-9_.-]+" required autoFocus /></label><label>Password<input name="password" type="password" minLength={12} autoComplete={registering ? 'new-password' : 'current-password'} required /></label><ActionButton type="submit" size="lg" loading={busy}>{registering ? tr('Daftar', 'Register') : tr('Masuk', 'Sign in')}</ActionButton><button type="button" className="text-button login-switch" onClick={() => { setRegistering(!registering); setError(''); }}>{registering ? tr('Sudah punya akun? Masuk', 'Already registered? Sign in') : tr('Belum punya akun? Daftar', 'No account yet? Register')}</button></form></main>;
}

function ChangePassword({ session, onDone, onError }: { session: Session; onDone: () => void; onError: (error: unknown) => void }) {
  const { tr } = useI18n(); const [busy,setBusy]=useState(false); const submit=async(event:FormEvent<HTMLFormElement>)=>{event.preventDefault();setBusy(true);const data=new FormData(event.currentTarget);try{await api('/api/v1/auth/change-password',{method:'POST',body:JSON.stringify({currentPassword:data.get('current'),newPassword:data.get('next')})});onDone();}catch(error){onError(error);}finally{setBusy(false);}};return <main className="login-screen password-change-screen"><section className="surface login-card"><LockKeyhole /><h2>{tr('Ganti password sementara', 'Change temporary password')}</h2><p>{tr(`Halo ${session.username}, tetapkan password baru sebelum melanjutkan.`, `Hello ${session.username}, set a new password before continuing.`)}</p><form className="form-stack" onSubmit={submit}><label>{tr('Password sementara', 'Temporary password')}<input name="current" type="password" autoComplete="current-password" required /></label><label>{tr('Password baru', 'New password')}<input name="next" type="password" minLength={12} autoComplete="new-password" required /></label><ActionButton type="submit" loading={busy}>{tr('Ganti password', 'Change password')}</ActionButton></form></section></main>;
}
