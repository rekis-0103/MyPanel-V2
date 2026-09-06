import { Archive, Boxes, Check, Cpu, HardDrive, MemoryStick, PackagePlus, RefreshCw, ShoppingCart, Sparkles, TrendingUp } from 'lucide-react';
import { useCallback, useEffect, useState, type CSSProperties, type FormEvent } from 'react';
import { api } from '../api';
import { ActionButton } from '../components/ui/ActionButton';
import { Modal } from '../components/ui/Modal';
import { EmptyState, Skeleton } from '../components/ui/States';
import { MetricBar } from '../components/ui/MetricBar';
import { useI18n } from '../i18n';
import { formatBytes, formatDate } from '../format';
import type { Capacity, HostingPackage, Order, PackageIcon, PanelUser, Runtime, Subscription } from '../types';
import grassIcon from '../assets/marketplace/grass.svg';
import anvilIcon from '../assets/marketplace/anvil.svg';
import goldIcon from '../assets/marketplace/gold-bar.svg';
import diamondIcon from '../assets/marketplace/cut-diamond.svg';
import featherIcon from '../assets/marketplace/feather.svg';
import crystalIcon from '../assets/marketplace/crystal-growth.svg';

const idr = new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 });

const marketplaceIcons = { grass: grassIcon, anvil: anvilIcon, gold: goldIcon, diamond: diamondIcon, feather: featherIcon, crystal: crystalIcon } as const;
const packageIconOptions: Array<{ id: PackageIcon; label: string }> = [
  { id: 'grass', label: 'Grass' },
  { id: 'anvil', label: 'Anvil' },
  { id: 'gold', label: 'Gold' },
  { id: 'diamond', label: 'Diamond' },
  { id: 'feather', label: 'Feather' },
  { id: 'crystal', label: 'Crystal' },
];
type PackagePresentation = { themeColor: string; icon: PackageIcon; isPopular: boolean; isRecommended: boolean };
const defaultPackagePresentation: PackagePresentation = { themeColor: '#3FB950', icon: 'grass', isPopular: false, isRecommended: false };
const packageLooks: Record<string, { icon: PackageIcon; themeColor: string }> = {
  starter: { icon: 'grass', themeColor: '#3FB950' },
  iron: { icon: 'anvil', themeColor: '#AEB7C2' },
  gold: { icon: 'gold', themeColor: '#E3B341' },
  diamond: { icon: 'diamond', themeColor: '#58C7DF' },
};
const fallbackVersions = [
  { id: '26.2', java: 25 as const },
  { id: '26.1', java: 25 as const },
  { id: '1.21.11', java: 21 as const },
  { id: '1.21.4', java: 21 as const },
  { id: '1.21.1', java: 21 as const },
];

function packageLook(item: HostingPackage) {
  const fallback = packageLooks[item.slug.toLowerCase()] ?? packageLooks.starter;
  return { icon: item.icon || fallback.icon, themeColor: item.themeColor || fallback.themeColor };
}

function runtimeIcon(runtime: Runtime) {
  return marketplaceIcons[runtime.icon ?? 'grass'];
}

function packageStyle(themeColor: string) {
  return { '--package-color': themeColor } as CSSProperties;
}

export function Marketplace({ catalog, reloadServers, notify, onError }: { catalog: Runtime[]; reloadServers: () => Promise<void>; notify: (message: string, variant?: 'success' | 'error' | 'info' | 'warning') => void; onError: (error: unknown) => void }) {
  const { tr } = useI18n();
  const [packages, setPackages] = useState<HostingPackage[] | null>(null);
  const [capacity, setCapacity] = useState<Capacity | null>(null);
  const [selected, setSelected] = useState<HostingPackage | null>(null);
  const [busy, setBusy] = useState(false);
  const [runtimeId, setRuntimeId] = useState('');
  const [version, setVersion] = useState('');
  const load = useCallback(() => Promise.all([api<HostingPackage[]>('/api/v1/packages'), api<Capacity>('/api/v1/capacity')]).then(([p, c]) => { setPackages(p); setCapacity(c); }).catch(onError), [onError]);
  useEffect(() => { load(); }, [load]);
  const selectedRuntime = catalog.find((runtime) => runtime.id === runtimeId) ?? catalog[0];
  const versions = selectedRuntime?.versions?.length ? selectedRuntime.versions : fallbackVersions;
  const selectedVersion = versions.find((item) => item.id === version) ?? versions[0];

  useEffect(() => {
    if (!selected || catalog.length === 0) return;
    const firstRuntime = catalog[0];
    const initialVersions = firstRuntime.versions?.length ? firstRuntime.versions : fallbackVersions;
    setRuntimeId(firstRuntime.id);
    setVersion(initialVersions[0].id);
  }, [selected, catalog]);

  const chooseRuntime = (runtime: Runtime) => {
    const runtimeVersions = runtime.versions?.length ? runtime.versions : fallbackVersions;
    setRuntimeId(runtime.id);
    setVersion(runtimeVersions[0].id);
  };

  const checkout = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!selected || !selectedRuntime || !selectedVersion) return;
    setBusy(true);
    const data = new FormData(event.currentTarget);
    try {
      await api('/api/v1/checkout', { method: 'POST', body: JSON.stringify({ packageId: selected.id, idempotencyKey: crypto.randomUUID(), name: data.get('name'), runtime: selectedRuntime.id, version: selectedVersion.id, javaVersion: selectedVersion.java }) });
      notify(tr('Pembayaran simulasi berhasil dan provisioning dimulai.', 'Simulated payment succeeded and provisioning started.'), 'success');
      setSelected(null);
      await Promise.all([load(), reloadServers()]);
    } catch (error) { onError(error); } finally { setBusy(false); }
  };
  return <div className="page-stack"><div className="page-heading"><div><p className="eyebrow">MARKETPLACE</p><h1>{tr('Beli Server', 'Buy a Server')}</h1><p>{tr('Pilih paket. Pembayaran hanya simulasi dan tidak memproses uang nyata.', 'Choose a package. Payment is simulated and never processes real money.')}</p></div></div>
    {packages === null ? <section className="surface"><Skeleton lines={4} /></section> : packages.length === 0 ? <section className="surface"><EmptyState icon={Boxes} title={tr('Belum ada paket aktif', 'No active packages')} description={tr('Administrator perlu mengaktifkan paket sebelum checkout tersedia.', 'An administrator must activate a package before checkout is available.')} /></section> : <div className="package-grid">{packages.map((item) => {
      const available = capacity?.packageAvailability[item.id] ?? false;
      const look = packageLook(item);
      return <article className={`surface package-card ${available ? '' : 'unavailable-card'}`} style={packageStyle(look.themeColor)} key={item.id}>
        <div className="package-card-top"><div className="package-icon"><img src={marketplaceIcons[look.icon]} alt="" /></div><div className="package-badges">{item.isPopular && <span className="popular"><TrendingUp />{tr('Paling laris', 'Best seller')}</span>}{item.isRecommended && <span className="recommended"><Sparkles />{tr('Rekomendasi', 'Recommended')}</span>}<span className="capacity-badge">{item.cpu} vCPU</span></div></div>
        <h2>{item.name}</h2><p>{item.description}</p><strong>{idr.format(item.priceIdr)}<small>/30 hari</small></strong>
        <dl><div><Cpu /><dt>{item.cpu} vCPU</dt></div><div><MemoryStick /><dt>{formatBytes(item.memoryMb * 1024 * 1024)} RAM</dt></div><div><HardDrive /><dt>{formatBytes(item.diskMb * 1024 * 1024)} disk</dt></div></dl>
        <ActionButton icon={ShoppingCart} disabled={!available} onClick={() => setSelected(item)}>{available ? tr('Pilih paket', 'Choose package') : tr('Kapasitas tidak cukup', 'Insufficient capacity')}</ActionButton>
      </article>;
    })}</div>}
    <Modal open={!!selected} onClose={() => !busy && setSelected(null)} title={tr(`Checkout ${selected?.name ?? ''}`, `Checkout ${selected?.name ?? ''}`)}>
      <form className="form-stack checkout-form" onSubmit={checkout}>
        <div className="simulated-payment"><Check /><span><b>{tr('Pembayaran simulasi instan', 'Instant simulated payment')}</b><small>{selected && idr.format(selected.priceIdr)} · 30 {tr('hari', 'days')}</small></span></div>
        <label>{tr('Nama server', 'Server name')}<input name="name" required maxLength={48} autoComplete="off" /></label>
        <fieldset className="runtime-picker"><legend>{tr('Jenis server', 'Server type')}</legend><div>{catalog.map((runtime) => <button type="button" key={runtime.id} className={runtime.id === selectedRuntime?.id ? 'selected' : ''} aria-pressed={runtime.id === selectedRuntime?.id} onClick={() => chooseRuntime(runtime)}><span><img src={runtimeIcon(runtime)} alt="" /></span>{runtime.name}</button>)}</div></fieldset>
        <div className="form-row"><label>{tr('Versi Minecraft', 'Minecraft version')}<select name="version" value={selectedVersion?.id ?? ''} onChange={(event) => setVersion(event.target.value)}>{versions.map((item) => <option key={item.id} value={item.id}>{item.id}</option>)}</select></label><label>Java<div className="java-version-display">Java {selectedVersion?.java ?? 21}<small>{tr('Dipilih otomatis', 'Selected automatically')}</small></div></label></div>
        <p className="form-note">{tr('Versi Java mengikuti kebutuhan versi Minecraft dan diverifikasi ulang oleh server.', 'Java follows the Minecraft version requirement and is verified again by the server.')}</p>
        <ActionButton type="submit" loading={busy} disabled={!selectedRuntime}>{tr('Bayar simulasi & buat server', 'Simulate payment & create server')}</ActionButton>
      </form>
    </Modal>
  </div>;
}

export function Billing({ notify, onError }: { notify: (message: string, variant?: 'success' | 'error' | 'info' | 'warning') => void; onError: (error: unknown) => void }) {
  const { tr } = useI18n(); const [orders, setOrders] = useState<Order[] | null>(null); const [subscriptions, setSubscriptions] = useState<Subscription[] | null>(null); const [busy, setBusy] = useState('');
  const load = useCallback(() => Promise.all([api<Order[]>('/api/v1/orders'), api<Subscription[]>('/api/v1/subscriptions')]).then(([o, s]) => { setOrders(o); setSubscriptions(s); }).catch(onError), [onError]); useEffect(() => { load(); }, [load]);
  const renew = async (item: Subscription) => { setBusy(item.id); try { const retry = item.status === 'action_required'; await api(`/api/v1/subscriptions/${item.id}/${retry ? 'retry' : 'renew'}`, { method: 'POST', body: JSON.stringify(retry ? {} : { idempotencyKey: crypto.randomUUID() }) }); notify(retry ? tr('Provisioning dicoba kembali.', 'Provisioning retry queued.') : tr('Langganan diperpanjang 30 hari.', 'Subscription renewed for 30 days.'), 'success'); await load(); } catch (error) { onError(error); } finally { setBusy(''); } };
  return <div className="page-stack"><div className="page-heading"><div><p className="eyebrow">SIMULATED BILLING</p><h1>{tr('Pesanan & Langganan', 'Orders & Subscriptions')}</h1><p>{tr('Seluruh transaksi di halaman ini adalah simulasi.', 'Every transaction on this page is simulated.')}</p></div></div>
    <section className="surface"><div className="section-heading"><div><h2>{tr('Langganan', 'Subscriptions')}</h2><p>{tr('Masa aktif dan status server', 'Server periods and status')}</p></div></div>{subscriptions === null ? <Skeleton lines={3} /> : subscriptions.length === 0 ? <EmptyState icon={RefreshCw} title={tr('Belum ada langganan', 'No subscriptions yet')} description={tr('Beli paket untuk membuat langganan pertama.', 'Buy a package to create your first subscription.')} /> : <div className="data-table hosting-table">{subscriptions.map((item) => { const retry = item.status === 'action_required'; const released = item.status === 'released'; const actionable = retry || item.status === 'active' || item.status === 'grace' || released; const statusNote = item.status === 'grace' ? tr(`Masa tenggang sampai ${formatDate(item.graceEndsAt)}`, `Grace period until ${formatDate(item.graceEndsAt)}`) : released ? tr('Resource komputasi dilepas; data tetap disimpan.', 'Compute resources released; data is retained.') : `${item.status} · ${tr('berakhir', 'ends')} ${formatDate(item.currentPeriodEnd)}`; return <article key={item.id}><div><b>{item.packageName}</b><small>{statusNote}</small></div><span>{item.cpu} vCPU · {formatBytes(item.memoryMb * 1024 * 1024)}</span>{actionable && <ActionButton size="sm" loading={busy === item.id} onClick={() => renew(item)}>{retry ? tr('Coba lagi', 'Retry') : released ? tr('Aktifkan lagi', 'Reactivate') : tr('Perpanjang', 'Renew')}</ActionButton>}</article>; })}</div>}</section>
    <section className="surface"><div className="section-heading"><div><h2>{tr('Riwayat Pesanan', 'Order History')}</h2><p>{tr('Bukti transaksi simulasi', 'Simulated transaction records')}</p></div></div>{orders === null ? <Skeleton lines={4} /> : orders.length === 0 ? <EmptyState icon={ShoppingCart} title={tr('Belum ada pesanan', 'No orders yet')} description={tr('Pesanan muncul setelah checkout.', 'Orders appear after checkout.')} /> : <div className="data-table hosting-table">{orders.map((item) => <article key={item.id}><div><b>{item.packageName} · {idr.format(item.amountIdr)}</b><small>{item.paymentReference} · {formatDate(item.createdAt)}</small></div><span>{item.kind} · {item.status}</span></article>)}</div>}</section>
  </div>;
}

export function AdminUsers({ notify, onError }: { notify: (message: string, variant?: 'success' | 'error' | 'info' | 'warning') => void; onError: (error: unknown) => void }) {
  const { tr } = useI18n(); const [items, setItems] = useState<PanelUser[] | null>(null); const [resetUser, setResetUser] = useState<PanelUser | null>(null); const [busy, setBusy] = useState(false);
  const load = useCallback(() => api<PanelUser[]>('/api/v1/admin/users').then(setItems).catch(onError), [onError]); useEffect(() => { load(); }, [load]);
  const status = async (item: PanelUser) => { setBusy(true); try { await api(`/api/v1/admin/users/${item.id}/status`, { method: 'PUT', body: JSON.stringify({ status: item.status === 'active' ? 'suspended' : 'active' }) }); notify(tr('Status akun diperbarui.', 'Account status updated.'), 'success'); await load(); } catch (error) { onError(error); } finally { setBusy(false); } };
  const reset = async (event: FormEvent<HTMLFormElement>) => { event.preventDefault(); if (!resetUser) return; setBusy(true); const data = new FormData(event.currentTarget); try { await api(`/api/v1/admin/users/${resetUser.id}/reset-password`, { method: 'POST', body: JSON.stringify({ password: data.get('password') }) }); notify(tr('Password sementara ditetapkan; session lama dibatalkan.', 'Temporary password set; previous sessions invalidated.'), 'success'); setResetUser(null); await load(); } catch (error) { onError(error); } finally { setBusy(false); } };
  return <div className="page-stack"><div className="page-heading"><div><p className="eyebrow">ADMIN</p><h1>{tr('Akun User', 'User Accounts')}</h1><p>{tr('Kelola akses tanpa melihat password user.', 'Manage access without viewing user passwords.')}</p></div></div><section className="surface">{items === null ? <Skeleton lines={5} /> : <div className="data-table hosting-table">{items.map((item) => <article key={item.id}><div><b>{item.username}</b><small>{item.id} · {item.role} · {item.status}</small></div>{item.role === 'user' && <div className="row-actions"><ActionButton size="sm" variant="secondary" onClick={() => setResetUser(item)}>{tr('Reset password', 'Reset password')}</ActionButton><ActionButton size="sm" variant={item.status === 'active' ? 'danger' : 'primary'} loading={busy} onClick={() => status(item)}>{item.status === 'active' ? tr('Suspend', 'Suspend') : tr('Aktifkan', 'Activate')}</ActionButton></div>}</article>)}</div>}</section><Modal open={!!resetUser} onClose={() => setResetUser(null)} title={tr(`Reset password ${resetUser?.username ?? ''}`, `Reset ${resetUser?.username ?? ''} password`)}><form className="form-stack" onSubmit={reset}><label>{tr('Password sementara', 'Temporary password')}<input name="password" type="password" minLength={12} required autoComplete="new-password" /></label><p className="form-note">{tr('User diwajibkan mengganti password setelah login.', 'The user must change this password after signing in.')}</p><ActionButton type="submit" loading={busy}>{tr('Tetapkan password', 'Set password')}</ActionButton></form></Modal></div>;
}

export function AdminPackages({ notify, onError }: { notify: (message: string, variant?: 'success' | 'error' | 'info' | 'warning') => void; onError: (error: unknown) => void }) {
  const { tr } = useI18n();
  const [items, setItems] = useState<HostingPackage[] | null>(null);
  const [editing, setEditing] = useState<HostingPackage | 'new' | null>(null);
  const [presentation, setPresentation] = useState<PackagePresentation>(defaultPackagePresentation);
  const [busy, setBusy] = useState(false);
  const load = useCallback(() => api<HostingPackage[]>('/api/v1/packages?all=1').then(setItems).catch(onError), [onError]);
  useEffect(() => { load(); }, [load]);

  const openEditor = (item: HostingPackage | 'new') => {
    setPresentation(item === 'new' ? defaultPackagePresentation : { themeColor: item.themeColor, icon: item.icon, isPopular: item.isPopular, isRecommended: item.isRecommended });
    setEditing(item);
  };
  const save = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const body = { slug: data.get('slug'), name: data.get('name'), description: data.get('description'), priceIdr: Number(data.get('price')), cpu: Number(data.get('cpu')), memoryMb: Number(data.get('memory')), diskMb: Number(data.get('disk')), sortOrder: Number(data.get('sort')), active: data.get('active') === 'on', ...presentation };
    setBusy(true);
    try {
      await api(editing === 'new' ? '/api/v1/packages' : `/api/v1/packages/${(editing as HostingPackage).id}`, { method: editing === 'new' ? 'POST' : 'PUT', body: JSON.stringify(body) });
      notify(tr('Paket disimpan.', 'Package saved.'), 'success');
      setEditing(null);
      await load();
    } catch (error) { onError(error); } finally { setBusy(false); }
  };
  const archive = async (item: HostingPackage) => { setBusy(true); try { await api(`/api/v1/packages/${item.id}`, { method: 'DELETE' }); notify(tr('Paket diarsipkan.', 'Package archived.'), 'success'); await load(); } catch (error) { onError(error); } finally { setBusy(false); } };
  const value = editing === 'new' ? null : editing;
  return <div className="page-stack">
    <div className="page-heading"><div><p className="eyebrow">ADMIN</p><h1>{tr('Paket Hosting', 'Hosting Packages')}</h1><p>{tr('Atur resource, tampilan, dan penanda promosi marketplace.', 'Configure marketplace resources, appearance, and promotional markers.')}</p></div><ActionButton icon={PackagePlus} onClick={() => openEditor('new')}>{tr('Paket baru', 'New package')}</ActionButton></div>
    <section className="surface">{items === null ? <Skeleton lines={5} /> : <div className="data-table hosting-table">{items.map((item) => <article className="admin-package-row" style={packageStyle(item.themeColor)} key={item.id}><div className="admin-package-logo"><img src={marketplaceIcons[item.icon]} alt="" /></div><div className="package-admin-copy"><b>{item.name} · {idr.format(item.priceIdr)}</b><small>{item.slug} · {item.cpu} vCPU · {formatBytes(item.memoryMb * 1024 * 1024)} · {item.active ? 'active' : 'archived'}</small><div className="package-badges">{item.isPopular && <span className="popular"><TrendingUp />{tr('Paling laris', 'Best seller')}</span>}{item.isRecommended && <span className="recommended"><Sparkles />{tr('Rekomendasi', 'Recommended')}</span>}</div></div><div className="row-actions"><ActionButton size="sm" variant="secondary" onClick={() => openEditor(item)}>{tr('Edit', 'Edit')}</ActionButton>{item.active && <ActionButton size="sm" variant="danger" icon={Archive} loading={busy} onClick={() => archive(item)}>{tr('Arsip', 'Archive')}</ActionButton>}</div></article>)}</div>}</section>
    <Modal open={!!editing} onClose={() => !busy && setEditing(null)} title={editing === 'new' ? tr('Paket baru', 'New package') : tr('Edit paket', 'Edit package')}>
      <form className="form-stack package-editor" onSubmit={save}>
        <div className="package-preview" style={packageStyle(presentation.themeColor)}><div className="package-icon"><img src={marketplaceIcons[presentation.icon]} alt="" /></div><div><b>{value?.name || tr('Preview paket', 'Package preview')}</b><div className="package-badges">{presentation.isPopular && <span className="popular"><TrendingUp />{tr('Paling laris', 'Best seller')}</span>}{presentation.isRecommended && <span className="recommended"><Sparkles />{tr('Rekomendasi', 'Recommended')}</span>}</div></div></div>
        <div className="form-row"><label>Slug<input name="slug" defaultValue={value?.slug} required pattern="[A-Za-z0-9_.-]{3,32}" /></label><label>{tr('Nama', 'Name')}<input name="name" defaultValue={value?.name} required maxLength={64} /></label></div>
        <label>{tr('Deskripsi', 'Description')}<textarea name="description" defaultValue={value?.description} maxLength={500} /></label>
        <fieldset className="package-icon-picker"><legend>{tr('Logo paket', 'Package logo')}</legend><div>{packageIconOptions.map((option) => <button key={option.id} type="button" className={presentation.icon === option.id ? 'selected' : ''} aria-pressed={presentation.icon === option.id} onClick={() => setPresentation((current) => ({ ...current, icon: option.id }))}><img src={marketplaceIcons[option.id]} alt="" /><span>{option.label}</span></button>)}</div></fieldset>
        <div className="form-row"><label>{tr('Warna tema', 'Theme color')}<div className="color-field"><input aria-label={tr('Pilih warna tema', 'Choose theme color')} type="color" value={presentation.themeColor} onChange={(event) => setPresentation((current) => ({ ...current, themeColor: event.target.value.toUpperCase() }))} /><code>{presentation.themeColor}</code></div></label><div className="package-marker-options"><span>{tr('Penanda marketplace', 'Marketplace markers')}</span><label className="checkbox-row"><input type="checkbox" checked={presentation.isPopular} onChange={(event) => setPresentation((current) => ({ ...current, isPopular: event.target.checked }))} />{tr('Paling laris', 'Best seller')}</label><label className="checkbox-row"><input type="checkbox" checked={presentation.isRecommended} onChange={(event) => setPresentation((current) => ({ ...current, isRecommended: event.target.checked }))} />{tr('Rekomendasi', 'Recommended')}</label></div></div>
        <div className="form-row"><label>{tr('Harga IDR', 'Price IDR')}<input name="price" type="number" min="0" defaultValue={value?.priceIdr ?? 29000} required /></label><label>vCPU<input name="cpu" type="number" min="1" max="32" defaultValue={value?.cpu ?? 1} required /></label></div>
        <div className="form-row"><label>RAM MiB<input name="memory" type="number" min="1024" defaultValue={value?.memoryMb ?? 1024} required /></label><label>Disk MiB<input name="disk" type="number" min="1024" defaultValue={value?.diskMb ?? 10240} required /></label></div>
        <div className="form-row"><label>Sort<input name="sort" type="number" defaultValue={value?.sortOrder ?? 10} /></label><label className="checkbox-row"><input name="active" type="checkbox" defaultChecked={value?.active ?? true} />Active</label></div>
        <ActionButton type="submit" loading={busy}>{tr('Simpan paket', 'Save package')}</ActionButton>
      </form>
    </Modal>
  </div>;
}

export function AdminCapacity({ onError }: { onError: (error: unknown) => void }) {
  const { tr } = useI18n(); const [capacity, setCapacity] = useState<Capacity | null>(null); useEffect(() => { api<Capacity>('/api/v1/capacity').then(setCapacity).catch(onError); }, [onError]); if (!capacity?.total || !capacity.reserved) return <section className="surface"><Skeleton lines={5} /></section>;
  const { total, reserved, available } = capacity; return <div className="page-stack"><div className="page-heading"><div><p className="eyebrow">ADMIN</p><h1>{tr('Kapasitas Node', 'Node Capacity')}</h1><p>{tr('Kapasitas jual menentukan apakah checkout dapat diterima.', 'Sellable capacity determines whether checkout can be accepted.')}</p></div></div><section className="surface capacity-panel"><MetricBar label="vCPU" value={reserved.cpu} max={total.cpu} unit="" detail={`${available.cpu} vCPU ${tr('tersisa', 'remaining')}`} /><MetricBar label="RAM" value={reserved.memoryMb} max={total.memoryMb} unit=" MiB" detail={`${formatBytes(available.memoryMb * 1024 * 1024)} ${tr('tersisa', 'remaining')}`} /><MetricBar label="Disk" value={reserved.diskMb} max={total.diskMb} unit=" MiB" detail={`${formatBytes(available.diskMb * 1024 * 1024)} ${tr('tersisa', 'remaining')}`} /><MetricBar label="Port" value={reserved.ports} max={total.ports} unit="" detail={`${available.ports} ${tr('tersisa', 'remaining')}`} />{capacity.host && <div className="host-facts"><span><Cpu />{capacity.host.cpu} CPU host</span><span><MemoryStick />{formatBytes(capacity.host.memoryAvailableBytes)} RAM available</span><span><HardDrive />{formatBytes(capacity.host.diskAvailableBytes)} disk available</span></div>}</section></div>;
}
