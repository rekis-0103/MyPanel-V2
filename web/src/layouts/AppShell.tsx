import { Activity, Bell, Boxes, ChevronRight, CreditCard, DatabaseBackup, FileText, Gauge, Languages, LogOut, Menu, Monitor, Moon, Package, Server as ServerIcon, Settings, ShoppingCart, Sun, Terminal, Users, X, Zap } from 'lucide-react';
import { useEffect, useState, type ReactNode } from 'react';
import { NavLink, useLocation, useNavigate } from 'react-router-dom';
import { useI18n } from '../i18n';
import type { Server, Session } from '../types';
import { StatusBadge, normalizeStatus } from '../components/ui/StatusBadge';

export function AppShell({ children, servers, session, onLogout }: { children: ReactNode; servers: Server[]; session: Session; onLogout: () => Promise<void> }) {
  const { locale, setLocale, tr } = useI18n();
  const location = useLocation();
  const navigate = useNavigate();
  const [moreOpen, setMoreOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => localStorage.getItem('mypanel.sidebar') === 'collapsed');
  const [theme, setTheme] = useState<'dark' | 'light'>(() => localStorage.getItem('mypanel.theme') === 'light' ? 'light' : 'dark');
  const routeServerId = location.pathname.match(/^\/servers\/([^/]+)/)?.[1];
  const activeServer = servers.find((server) => server.id === routeServerId) ?? null;
  useEffect(() => { document.documentElement.dataset.theme = theme; localStorage.setItem('mypanel.theme', theme); }, [theme]);
  useEffect(() => { localStorage.setItem('mypanel.sidebar', sidebarCollapsed ? 'collapsed' : 'expanded'); }, [sidebarCollapsed]);
  const serverPath = (page: string) => activeServer ? `/servers/${activeServer.id}/${page}` : '/servers';
  const alerts = servers.filter((server) => server.lastError || server.state === 'error');
  const allocated = servers.reduce((total, server) => ({ cpu: total.cpu + server.cpu, memory: total.memory + server.memoryMb, disk: total.disk + server.diskMb }), { cpu: 0, memory: 0, disk: 0 });
  const globalNav = [
    { to: '/dashboard', label: tr('Dashboard', 'Dashboard'), icon: Gauge },
    { to: '/servers', label: tr('Server', 'Servers'), icon: ServerIcon },
    ...(session.role === 'user' ? [{ to: '/marketplace', label: tr('Beli Server', 'Buy Server'), icon: ShoppingCart }] : []),
    { to: '/billing', label: tr('Pesanan', 'Orders'), icon: CreditCard },
    ...(session.role === 'owner' ? [
      { to: '/admin/users', label: tr('User', 'Users'), icon: Users },
      { to: '/admin/packages', label: tr('Paket', 'Packages'), icon: Package },
      { to: '/admin/capacity', label: tr('Kapasitas', 'Capacity'), icon: Monitor },
      { to: '/activity', label: tr('Log Aktivitas', 'Activity Log'), icon: Activity },
    ] : []),
  ];
  const contextualNav = activeServer ? [
    { to: serverPath('console'), label: tr('Console', 'Console'), icon: Terminal },
    { to: serverPath('files'), label: tr('File', 'Files'), icon: FileText },
    { to: serverPath('backups'), label: tr('Backup', 'Backups'), icon: DatabaseBackup },
    { to: serverPath('schedules'), label: tr('Jadwal', 'Schedules'), icon: Zap },
    { to: serverPath('settings'), label: tr('Pengaturan', 'Settings'), icon: Settings },
  ] : [];
  const nav = [...globalNav, ...contextualNav];
  const crumbs = breadcrumb(location.pathname, activeServer?.name, tr);
  return <div className={sidebarCollapsed ? 'app-shell sidebar-collapsed' : 'app-shell'}>
    <aside className="sidebar" aria-label={tr('Navigasi utama', 'Main navigation')}>
      <div className="sidebar-head"><button className="brand" onClick={() => navigate('/dashboard')} aria-label="MyPanel dashboard"><span className="brand-mark"><Boxes /></span><span className="brand-copy">MyPanel<small>v2</small></span></button><button className="icon-button sidebar-toggle" onClick={() => setSidebarCollapsed((value) => !value)} aria-label={sidebarCollapsed ? tr('Tampilkan sidebar', 'Expand sidebar') : tr('Sembunyikan sidebar', 'Collapse sidebar')} title={sidebarCollapsed ? tr('Tampilkan sidebar', 'Expand sidebar') : tr('Sembunyikan sidebar', 'Collapse sidebar')}><Menu /></button></div>
      <nav className="global-nav">{globalNav.map(({ to, label, icon: Icon }) => <NavLink key={label} to={to} end={to === '/servers'} title={label} className={({ isActive }) => isActive ? 'active' : ''}><Icon /><span>{label}</span></NavLink>)}</nav>
      {activeServer && <section className="server-nav-panel" aria-label={tr(`Menu server ${activeServer.name}`, `${activeServer.name} server menu`)}><header title={activeServer.name}><span className="server-nav-icon"><ServerIcon /></span><div><small>{tr('SERVER AKTIF', 'ACTIVE SERVER')}</small><b>{activeServer.name}</b></div><StatusBadge status={normalizeStatus(activeServer.state)} compact /></header><nav>{contextualNav.map(({ to, label, icon: Icon }) => <NavLink key={label} to={to} title={label} className={({ isActive }) => isActive ? 'active' : ''}><Icon /><span>{label}</span></NavLink>)}</nav></section>}
      {session.role === 'owner' && <section className="node-mini" aria-label={tr('Informasi node', 'Node information')}>
        <div><Monitor /><span><b>local</b><small>{tr('Telemetri belum tersedia', 'Telemetry unavailable')}</small></span></div>
        <dl><div><dt>CPU</dt><dd>{allocated.cpu} vCPU</dd></div><div><dt>RAM</dt><dd>{formatAllocation(allocated.memory)}</dd></div><div><dt>Disk</dt><dd>{formatAllocation(allocated.disk)}</dd></div></dl>
      </section>}
    </aside>
    <div className="app-column">
      <header className="topbar">
        <div className="breadcrumbs topbar-crumbs">{crumbs.map((item, index) => <span key={`${item}-${index}`}>{index > 0 && <ChevronRight />}{item}</span>)}</div>
        <div className="topbar-actions">
          {activeServer && <StatusBadge status={normalizeStatus(activeServer.state)} />}
          {alerts.length > 0 && <button className="icon-button notification" title={tr(`${alerts.length} server memerlukan perhatian`, `${alerts.length} servers need attention`)} onClick={() => navigate('/servers')}><Bell /><span>{alerts.length}</span></button>}
          <button className="icon-button theme-toggle" onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')} title={theme === 'dark' ? tr('Gunakan mode terang', 'Use light mode') : tr('Gunakan mode gelap', 'Use dark mode')} aria-label={theme === 'dark' ? tr('Gunakan mode terang', 'Use light mode') : tr('Gunakan mode gelap', 'Use dark mode')}>{theme === 'dark' ? <Sun /> : <Moon />}</button>
          <button className="language-toggle" onClick={() => setLocale(locale === 'id' ? 'en' : 'id')} aria-label={tr('Ubah bahasa ke Inggris', 'Change language to Indonesian')}><Languages />{locale.toUpperCase()}</button>
          <div className="user-chip" title={`${session.username} · ${session.role}`}><span>{session.username.slice(0, 2).toUpperCase()}</span><div><b>{session.username}</b><small>{session.role}</small></div></div>
          <button className="icon-button logout" onClick={onLogout} title={tr('Keluar', 'Sign out')}><LogOut /></button>
        </div>
      </header>
      <main className="main-content">{children}</main>
    </div>
    <nav className="mobile-nav" aria-label={tr('Navigasi seluler', 'Mobile navigation')}>
      {nav.slice(0, 4).map(({ to, label, icon: Icon }) => <NavLink key={label} to={to} end={to === '/servers'}><Icon /><span>{label}</span></NavLink>)}
      <button onClick={() => setMoreOpen(true)}><Menu /><span>{tr('Lainnya', 'More')}</span></button>
    </nav>
    {moreOpen && <div className="mobile-more-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) setMoreOpen(false); }}><section className="mobile-more"><header><b>{tr('Menu lainnya', 'More')}</b><button className="icon-button" onClick={() => setMoreOpen(false)}><X /></button></header>{nav.slice(4).map(({ to, label, icon: Icon }) => <NavLink key={label} to={to} onClick={() => setMoreOpen(false)}><Icon />{label}</NavLink>)}<button onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}>{theme === 'dark' ? <Sun /> : <Moon />}{theme === 'dark' ? tr('Mode terang', 'Light mode') : tr('Mode gelap', 'Dark mode')}</button><button onClick={() => setLocale(locale === 'id' ? 'en' : 'id')}><Languages />{tr('Gunakan English', 'Gunakan Indonesia')}</button><button onClick={onLogout}><LogOut />{tr('Keluar', 'Sign out')}</button></section></div>}
  </div>;
}

function breadcrumb(path: string, serverName: string | undefined, tr: (id: string, en: string) => string) {
  if (path === '/dashboard') return [tr('Dashboard', 'Dashboard')];
  if (path === '/activity') return [tr('Log Aktivitas', 'Activity Log')];
  if (path === '/marketplace') return [tr('Beli Server', 'Buy Server')];
  if (path === '/billing') return [tr('Pesanan', 'Orders')];
  if (path === '/admin/users') return [tr('Admin', 'Admin'), tr('User', 'Users')];
  if (path === '/admin/packages') return [tr('Admin', 'Admin'), tr('Paket', 'Packages')];
  if (path === '/admin/capacity') return [tr('Admin', 'Admin'), tr('Kapasitas', 'Capacity')];
  const parts = path.split('/').filter(Boolean);
  if (parts[0] !== 'servers') return ['MyPanel'];
  if (parts.length === 1) return [tr('Server', 'Servers')];
  const labels: Record<string, string> = { console: 'Console', files: tr('File', 'Files'), backups: tr('Backup', 'Backups'), schedules: tr('Jadwal', 'Schedules'), settings: tr('Pengaturan', 'Settings') };
  return [tr('Server', 'Servers'), serverName ?? tr('Tidak diketahui', 'Unknown'), labels[parts[2]] ?? 'Console'];
}

function formatAllocation(mb: number) { return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GiB` : `${mb} MiB`; }
