import { Blocks, Download, RefreshCw, Search, Trash2 } from 'lucide-react';
import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { api, ApiError } from '../api';
import { ActionButton } from '../components/ui/ActionButton';
import { Modal } from '../components/ui/Modal';
import { EmptyState, Skeleton } from '../components/ui/States';
import { useI18n } from '../i18n';
import type { Addon, AddonSearchResult, Server } from '../types';

export function Addons({ server, notify, onError }: { server: Server; notify: (message: string, variant?: 'success' | 'error' | 'info' | 'warning') => void; onError: (error: unknown) => void }) {
  const { tr } = useI18n();
  const [installed, setInstalled] = useState<Addon[] | null>(null);
  const [results, setResults] = useState<AddonSearchResult[] | null>(null);
  const [provider, setProvider] = useState<'modrinth' | 'curseforge'>('modrinth');
  const [busy, setBusy] = useState('');
  const [dependencyTarget, setDependencyTarget] = useState<{ result: AddonSearchResult; addonId?: string } | null>(null);
  const supported = ['paper', 'purpur', 'fabric'].includes(server.runtime);
  const load = useCallback(() => {
    if (!supported) { setInstalled([]); return Promise.resolve(); }
    return api<Addon[]>(`/api/v1/servers/${server.id}/addons`).then(setInstalled).catch(onError);
  }, [onError, server.id, supported]);
  useEffect(() => { load(); const timer = window.setInterval(load, 5000); return () => window.clearInterval(timer); }, [load]);

  const search = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); const query = String(new FormData(event.currentTarget).get('query') ?? '').trim(); if (query.length < 2) return; setBusy('search'); setResults(null);
    try { const response = await api<{ items: AddonSearchResult[] }>(`/api/v1/servers/${server.id}/addons/search?provider=${provider}&q=${encodeURIComponent(query)}`); setResults(response.items); }
    catch (error) { setResults([]); onError(error); } finally { setBusy(''); }
  };
  const install = async (item: AddonSearchResult, confirmDependencies = false, addonId = '') => {
    setBusy(addonId || item.projectId);
    try { await api(`/api/v1/servers/${server.id}/addons`, { method: 'POST', body: JSON.stringify({ provider: item.provider, projectId: item.projectId, confirmDependencies, addonId }) }); notify(addonId ? tr('Pembaruan add-on masuk antrean.', 'Add-on update queued.') : tr('Instalasi add-on masuk antrean.', 'Add-on installation queued.'), 'success'); setDependencyTarget(null); await load(); }
    catch (error) { if (error instanceof ApiError && error.code === 'dependencies_required') setDependencyTarget({ result: item, addonId }); else onError(error); }
    finally { setBusy(''); }
  };
  const update = (item: Addon) => install({ provider: item.provider, projectId: item.projectId, name: item.name, description: '', iconUrl: '', downloads: 0 }, false, item.id);
  const remove = async (item: Addon) => { setBusy(item.id); try { await api(`/api/v1/servers/${server.id}/addons/${item.id}`, { method: 'DELETE' }); notify(tr('Penghapusan add-on masuk antrean.', 'Add-on removal queued.'), 'success'); await load(); } catch (error) { onError(error); } finally { setBusy(''); } };

  if (!supported) return <div className="page-stack"><div className="page-heading"><div><p className="eyebrow">MANAGED ADD-ONS</p><h1>Add-ons</h1></div></div><section className="surface"><EmptyState icon={Blocks} title={tr('Runtime ini tidak memuat add-on', 'This runtime does not load add-ons')} description={tr('Gunakan Paper atau Purpur untuk plugin, atau Fabric untuk mod terkelola.', 'Use Paper or Purpur for plugins, or Fabric for managed mods.')} /></section></div>;

  return <div className="page-stack"><div className="page-heading"><div><p className="eyebrow">MANAGED ADD-ONS</p><h1>{server.runtime === 'paper' || server.runtime === 'purpur' ? 'Plugin' : 'Mod'}</h1><p>{tr(`Cari versi yang cocok untuk ${server.runtime} ${server.version}; file diverifikasi sebelum dipasang.`, `Find versions compatible with ${server.runtime} ${server.version}; files are verified before installation.`)}</p></div>{server.restartRequired && <span className="restart-required"><RefreshCw />{tr('Restart diperlukan', 'Restart required')}</span>}</div>
    <section className="surface addon-browser"><form className="addon-search" onSubmit={search}><select aria-label={tr('Penyedia', 'Provider')} value={provider} onChange={(event) => setProvider(event.target.value as 'modrinth' | 'curseforge')}><option value="modrinth">Modrinth</option><option value="curseforge">CurseForge</option></select><input name="query" minLength={2} maxLength={80} required placeholder={tr('Cari plugin atau mod…', 'Search plugins or mods…')} /><ActionButton type="submit" icon={Search} loading={busy === 'search'}>{tr('Cari', 'Search')}</ActionButton></form>{results === null ? <EmptyState icon={Search} title={tr('Temukan add-on kompatibel', 'Find compatible add-ons')} description={tr('Pencarian otomatis difilter berdasarkan runtime dan versi Minecraft server.', 'Search is automatically filtered by the server runtime and Minecraft version.')} /> : results.length === 0 ? <EmptyState icon={Blocks} title={tr('Tidak ada hasil kompatibel', 'No compatible results')} description={tr('Coba nama lain atau penyedia berbeda.', 'Try another name or provider.')} /> : <div className="addon-results">{results.map((item) => <article key={`${item.provider}-${item.projectId}`}>{item.iconUrl ? <img src={item.iconUrl} alt="" loading="lazy" referrerPolicy="no-referrer" /> : <span><Blocks /></span>}<div><b>{item.name}</b><p>{item.description}</p><small>{item.provider} · {Intl.NumberFormat().format(item.downloads)} downloads</small></div><ActionButton size="sm" icon={Download} loading={busy === item.projectId} disabled={!!server.currentJob} onClick={() => install(item)}>{tr('Pasang', 'Install')}</ActionButton></article>)}</div>}</section>
    <section className="surface"><div className="section-heading"><div><h2>{tr('Terpasang', 'Installed')}</h2><p>{tr('File yang dikelola MyPanel', 'Files managed by MyPanel')}</p></div></div>{installed === null ? <Skeleton lines={3} /> : installed.length === 0 ? <EmptyState icon={Blocks} title={tr('Belum ada add-on terkelola', 'No managed add-ons yet')} description={tr('Cari di atas untuk memasang plugin atau mod pertama.', 'Search above to install your first plugin or mod.')} /> : <div className="data-list">{installed.map((item) => <article key={item.id}><Blocks /><div><b>{item.name}</b><small>{item.fileName} · {item.provider}</small></div><span className={`data-status ${item.status === 'installed' ? 'ready' : item.status}`}>{item.status}</span><div className="row-actions"><ActionButton size="sm" variant="ghost" icon={RefreshCw} loading={busy === item.id} disabled={!!server.currentJob} onClick={() => update(item)}>{tr('Perbarui', 'Update')}</ActionButton><ActionButton size="sm" variant="ghost" icon={Trash2} loading={busy === item.id} disabled={!!server.currentJob} onClick={() => remove(item)}>{tr('Hapus', 'Remove')}</ActionButton></div></article>)}</div>}</section>
    <Modal open={!!dependencyTarget} onClose={() => setDependencyTarget(null)} title={tr('Pasang dependency?', 'Install dependencies?')} footer={<><ActionButton variant="secondary" onClick={() => setDependencyTarget(null)}>{tr('Batal', 'Cancel')}</ActionButton><ActionButton loading={!!dependencyTarget && busy === (dependencyTarget.addonId || dependencyTarget.result.projectId)} onClick={() => dependencyTarget && install(dependencyTarget.result, true, dependencyTarget.addonId)}>{tr('Pasang semua', 'Install all')}</ActionButton></>}><p>{tr('Add-on ini membutuhkan dependency. MyPanel akan mengunduh semua file wajib dari penyedia yang sama dan memverifikasi checksum-nya.', 'This add-on requires dependencies. MyPanel will download all required files from the same provider and verify their checksums.')}</p></Modal>
  </div>;
}
