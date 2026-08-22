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
import { Backups } from './pages/Backups';
import { Dashboard } from './pages/Dashboard';
import { FileManager } from './pages/FileManager';
import { Schedules } from './pages/Schedules';
import { ServerSettings } from './pages/ServerSettings';
import { ServersPage } from './pages/Servers';
import type { ActionResponse, Runtime, Server, Session } from './types';

const ConsolePage = lazy(() => import('./pages/Console').then((module) => ({ default: module.Console })));

export function App() {
  const { tr } = useI18n(); const { showToast } = useToast(); const navigate = useNavigate();
  const [session, setCurrentSession] = useState<Session | null | undefined>(undefined); const [servers, setServers] = useState<Server[]>([]); const [catalog, setCatalog] = useState<Runtime[]>([]); const [busy, setBusy] = useState(false);
  const signOut = useCallback(() => { setApiSession(null); setCurrentSession(null); setServers([]); }, []);
  const handleError = useCallback((value: unknown) => { if (value instanceof ApiError && value.status === 401) { signOut(); return; } showToast(value instanceof Error ? value.message : tr('Terjadi kesalahan yang tidak diketahui.', 'An unknown error occurred.'), 'error'); }, [showToast, signOut, tr]);
  const loadServers = useCallback(async () => { try { setServers(await api<Server[]>('/api/v1/servers')); } catch (error) { handleError(error); } }, [handleError]);
  useEffect(() => { api<Session>('/api/v1/auth/me').then(async (me) => { setApiSession(me); const [runtimeItems, serverItems] = await Promise.all([api<Runtime[]>('/api/v1/catalog'), api<Server[]>('/api/v1/servers')]); setCatalog(runtimeItems); setServers(serverItems); setCurrentSession(me); }).catch(() => signOut()); }, [signOut]);
  useEffect(() => { if (!session) return; const timer = window.setInterval(loadServers, servers.some((server) => server.currentJob) ? 1500 : 5000); return () => window.clearInterval(timer); }, [loadServers, servers, session]);
  const act = async (server: Server, action: string) => { setBusy(true); try { await api<ActionResponse>(`/api/v1/servers/${server.id}/actions`, { method: 'POST', body: JSON.stringify({ action }) }); showToast(tr(`${action} masuk ke antrean.`, `${action} queued.`), 'success'); await loadServers(); } catch (error) { handleError(error); } finally { setBusy(false); } };
  const remove = async (server: Server, purgeData: boolean) => { setBusy(true); try { await api<ActionResponse>(`/api/v1/servers/${server.id}`, { method: 'DELETE', body: JSON.stringify({ purgeData }) }); showToast(tr('Penghapusan masuk ke antrean.', 'Deletion queued.'), 'success'); navigate('/servers'); await loadServers(); } catch (error) { handleError(error); } finally { setBusy(false); } };
  const notify = useCallback((message: string, variant: 'success' | 'error' | 'info' | 'warning' = 'info') => showToast(message, variant), [showToast]);
  if (session === undefined) return <LoadingScreen />;
  if (!session) return <Login onLogin={() => window.location.reload()} />;
  return <AppShell servers={servers} session={session} onLogout={async () => { try { await api('/api/v1/auth/logout', { method: 'POST' }); } finally { signOut(); } }}><Routes>
    <Route index element={<Navigate to="/dashboard" replace />} />
    <Route path="dashboard" element={<Page title={tr('Dashboard gagal dimuat.', 'Dashboard failed to load.')}><Dashboard servers={servers} busy={busy} act={act} onError={handleError} /></Page>} />
    <Route path="servers" element={<Page title={tr('Daftar server gagal dimuat.', 'Server list failed to load.')}><ServersPage servers={servers} catalog={catalog} busy={busy} act={act} setBusy={setBusy} onCreated={async () => { showToast(tr('Server dibuat dan provisioning dimulai.', 'Server created and provisioning started.'), 'success'); await loadServers(); }} onError={handleError} /></Page>} />
    <Route path="servers/:serverId" element={<ServerRedirect servers={servers} />} />
    <Route path="servers/:serverId/console" element={<ServerPage servers={servers}>{(server) => <Suspense fallback={<div className="surface"><span className="spinner" /></div>}><ConsolePage server={server} csrfToken={session.csrfToken} busy={busy} act={act} /></Suspense>}</ServerPage>} />
    <Route path="servers/:serverId/files" element={<ServerPage servers={servers}>{(server) => <FileManager server={server} notify={notify} onError={handleError} />}</ServerPage>} />
    <Route path="servers/:serverId/backups" element={<ServerPage servers={servers}>{(server) => <Backups server={server} notify={notify} onError={handleError} />}</ServerPage>} />
    <Route path="servers/:serverId/schedules" element={<ServerPage servers={servers}>{(server) => <Schedules server={server} notify={notify} onError={handleError} />}</ServerPage>} />
    <Route path="servers/:serverId/settings" element={<ServerPage servers={servers}>{(server) => <ServerSettings server={server} reload={loadServers} notify={notify} onError={handleError} remove={remove} />}</ServerPage>} />
    <Route path="activity" element={<Page title={tr('Log aktivitas gagal dimuat.', 'Activity log failed to load.')}><ActivityLog onError={handleError} /></Page>} />
    <Route path="*" element={<Navigate to="/dashboard" replace />} />
  </Routes></AppShell>;
}

function Page({ title, children }: { title: string; children: ReactNode }) { const { tr } = useI18n(); return <PageErrorBoundary message={title} reloadLabel={tr('Muat ulang', 'Reload')}>{children}</PageErrorBoundary>; }
function ServerRedirect({ servers }: { servers: Server[] }) { const { serverId } = useParams(); return servers.some((server) => server.id === serverId) ? <Navigate to={`/servers/${serverId}/console`} replace /> : <Navigate to="/servers" replace />; }
function ServerPage({ servers, children }: { servers: Server[]; children: (server: Server) => ReactNode }) { const { tr } = useI18n(); const { serverId } = useParams(); const server = servers.find((item) => item.id === serverId); if (!server) return <Navigate to="/servers" replace />; return <Page title={tr('Halaman server gagal dimuat.', 'Server page failed to load.')}>{children(server)}</Page>; }

function LoadingScreen() { const { tr } = useI18n(); return <main className="loading-screen"><div className="brand-emblem"><ServerIcon /></div><span className="spinner" /><p>{tr('Menghubungkan ke control plane…', 'Connecting to the control plane…')}</p></main>; }

export function Login({ onLogin }: { onLogin: (session: Session) => void }) {
  const { locale, setLocale, tr } = useI18n(); const [error, setError] = useState(''); const [busy, setBusy] = useState(false);
  const submit = async (event: FormEvent<HTMLFormElement>) => { event.preventDefault(); setBusy(true); setError(''); const data = new FormData(event.currentTarget); try { const session = await api<Session>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ username: data.get('username'), password: data.get('password') }) }); setApiSession(session); onLogin(session); } catch (value) { setError(value instanceof Error ? value.message : tr('Login gagal.', 'Login failed.')); } finally { setBusy(false); } };
  return <main className="login-screen"><button className="login-language language-toggle" onClick={() => setLocale(locale === 'id' ? 'en' : 'id')}>{locale.toUpperCase()}</button><section className="login-intro"><div className="login-brand"><span className="brand-mark"><ServerIcon /></span><b>MyPanel <small>v2</small></b></div><p className="eyebrow">MINECRAFT CONTROL PLANE</p><h1>{tr('Server Anda. Infrastruktur Anda.', 'Your servers. Your infrastructure.')}</h1><p>{tr('Kelola runtime, resource, file, backup, dan console Minecraft dari satu node privat.', 'Manage Minecraft runtimes, resources, files, backups, and consoles from one private node.')}</p></section><form className="surface login-card" onSubmit={submit}><LockKeyhole /><h2>{tr('Masuk ke MyPanel', 'Sign in to MyPanel')}</h2><p>{tr('Gunakan akun owner yang dibuat saat instalasi.', 'Use the owner account created during installation.')}</p>{error && <div className="inline-error" role="alert">{error}</div>}<label>Username<input name="username" autoComplete="username" required autoFocus /></label><label>Password<input name="password" type="password" autoComplete="current-password" required /></label><ActionButton type="submit" size="lg" loading={busy}>{tr('Masuk', 'Sign in')}</ActionButton></form></main>;
}
