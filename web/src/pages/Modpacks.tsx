import { Archive, CheckCircle2, Download, ExternalLink, HardDriveDownload, Search, ShieldAlert } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import { api, ApiError } from '../api';
import { formatDate } from '../format';
import { useI18n } from '../i18n';
import type { Modpack, ModpackSearchResult, ModpackVersion, Server } from '../types';
import { ActionButton } from '../components/ui/ActionButton';
import { Modal } from '../components/ui/Modal';
import { EmptyState, Skeleton } from '../components/ui/States';

export function Modpacks({ server, reload, notify, onError }: { server: Server; reload: () => Promise<void>; notify: (message: string, variant?: 'success' | 'error' | 'info' | 'warning') => void; onError: (error: unknown) => void }) {
  const { tr } = useI18n();
  const [installed, setInstalled] = useState<Modpack | null | undefined>(undefined);
  const [results, setResults] = useState<ModpackSearchResult[] | null>(null);
  const [selected, setSelected] = useState<ModpackSearchResult | null>(null);
  const [versions, setVersions] = useState<ModpackVersion[] | null>(null);
  const [fileId, setFileId] = useState('');
  const [busy, setBusy] = useState<'search' | 'versions' | 'install' | ''>('');
  const [providerError, setProviderError] = useState('');
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [confirmation, setConfirmation] = useState('');

  const loadInstalled = useCallback(async () => {
    try {
      const response = await api<{ item: Modpack | null }>(`/api/v1/servers/${server.id}/modpacks`);
      setInstalled(response.item);
    } catch (error) {
      onError(error);
    }
  }, [onError, server.id]);

  useEffect(() => {
    void loadInstalled();
    const timer = window.setInterval(loadInstalled, 5000);
    return () => window.clearInterval(timer);
  }, [loadInstalled]);

  const chosenVersion = useMemo(() => versions?.find((item) => item.fileId === fileId) ?? versions?.[0] ?? null, [fileId, versions]);

  const search = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const query = String(new FormData(event.currentTarget).get('query') ?? '').trim();
    if (query.length < 2) return;
    setBusy('search'); setProviderError(''); setResults(null); setSelected(null); setVersions(null);
    try {
      const response = await api<{ items: ModpackSearchResult[] }>(`/api/v1/servers/${server.id}/modpacks/search?q=${encodeURIComponent(query)}`);
      setResults(response.items);
    } catch (error) {
      setResults([]);
      if (error instanceof ApiError && error.code === 'provider_not_configured') setProviderError(tr('CurseForge belum dikonfigurasi oleh admin.', 'CurseForge has not been configured by an administrator.'));
      else onError(error);
    } finally {
      setBusy('');
    }
  };

  const choose = async (item: ModpackSearchResult) => {
    setSelected(item); setVersions(null); setFileId(''); setBusy('versions'); setProviderError('');
    try {
      const response = await api<{ items: ModpackVersion[] }>(`/api/v1/servers/${server.id}/modpacks/${item.projectId}/versions`);
      setVersions(response.items);
      setFileId(response.items[0]?.fileId ?? '');
    } catch (error) {
      setVersions([]); onError(error);
    } finally {
      setBusy('');
    }
  };

  const install = async () => {
    if (!selected || !chosenVersion || confirmation !== server.name) return;
    setBusy('install');
    try {
      await api(`/api/v1/servers/${server.id}/modpacks`, { method: 'POST', body: JSON.stringify({ projectId: selected.projectId, fileId: chosenVersion.fileId, confirmation }) });
      setConfirmOpen(false); setConfirmation('');
      notify(tr('Penggantian server ke modpack masuk antrean. Backup dibuat sebelum instalasi.', 'Modpack replacement queued. A backup is created before installation.'), 'success');
      await Promise.all([loadInstalled(), reload()]);
    } catch (error) {
      onError(error);
    } finally {
      setBusy('');
    }
  };

  return <div className="page-stack modpack-page">
    <div className="page-heading"><div><p className="eyebrow">CURSEFORGE MODPACKS</p><h1>{tr('Modpack Server', 'Server Modpacks')}</h1><p>{tr('Pilih file modpack yang kompatibel. MyPanel menyesuaikan Forge atau NeoForge, versi Minecraft, dan Java secara otomatis.', 'Choose a compatible modpack file. MyPanel automatically selects Forge or NeoForge, Minecraft, and Java versions.')}</p></div><span className="provider-lock"><ShieldAlert />CurseForge only</span></div>

    {server.currentJob?.action === 'modpack_install' && <div className="job-strip"><span className="spinner" /><div><b>{tr('Sedang memasang modpack', 'Installing modpack')}</b><small>{tr('Server dihentikan, dicadangkan, diganti, lalu diverifikasi sebelum job selesai.', 'The server is stopped, backed up, replaced, and verified before the job completes.')}</small></div></div>}

    <section className="surface modpack-current">
      <div className="section-heading"><div><h2>{tr('Modpack aktif', 'Active modpack')}</h2><p>{tr('Satu modpack terkelola per server', 'One managed modpack per server')}</p></div></div>
      {installed === undefined ? <Skeleton lines={2} /> : installed === null ? <EmptyState icon={Archive} title={tr('Belum menggunakan modpack', 'No modpack installed')} description={tr('Server saat ini tetap menggunakan runtime biasa. Cari CurseForge di bawah untuk menggantinya.', 'The server is still using its regular runtime. Search CurseForge below to replace it.')} /> : <article className="active-modpack">
        {installed.iconUrl ? <img src={installed.iconUrl} alt="" referrerPolicy="no-referrer" /> : <span><Archive /></span>}
        <div><small>{installed.provider}</small><h2>{installed.name}</h2><p>{installed.versionName}</p><dl><div><dt>Runtime</dt><dd>{installed.runtime}</dd></div><div><dt>Minecraft</dt><dd>{installed.minecraftVersion}</dd></div><div><dt>Java</dt><dd>{installed.javaVersion}</dd></div></dl></div>
        <span className={`data-status ${installed.status === 'installed' ? 'ready' : 'creating'}`}>{installed.status}</span>
      </article>}
    </section>

    <section className="surface modpack-browser">
      <form className="modpack-search" onSubmit={search}><div><b>CurseForge</b><small>{tr('Katalog resmi Minecraft Modpacks', 'Official Minecraft Modpacks catalog')}</small></div><input name="query" minLength={2} maxLength={80} required placeholder={tr('Cari modpack, misalnya All the Mods…', 'Search modpacks, for example All the Mods…')} /><ActionButton type="submit" icon={Search} loading={busy === 'search'}>{tr('Cari', 'Search')}</ActionButton></form>
      {providerError && <div className="modpack-provider-error" role="alert"><ShieldAlert /><div><b>{providerError}</b><p>{tr('Admin perlu memasang CurseForge API key sebagai secret pada controller.', 'An administrator must mount a CurseForge API key as a controller secret.')}</p></div></div>}
      {results === null ? <EmptyState icon={Search} title={tr('Temukan modpack', 'Find a modpack')} description={tr('Hanya file manifest Forge dan NeoForge yang kompatibel dengan Java panel yang akan ditampilkan.', 'Only Forge and NeoForge manifest files compatible with the panel Java versions are shown.')} /> : results.length === 0 && !providerError ? <EmptyState icon={Archive} title={tr('Modpack tidak ditemukan', 'No modpacks found')} description={tr('Coba nama atau kata kunci lain.', 'Try another name or keyword.')} /> : <div className="modpack-results">{results.map((item) => <button type="button" key={item.projectId} className={selected?.projectId === item.projectId ? 'selected' : ''} onClick={() => void choose(item)}><span>{item.iconUrl ? <img src={item.iconUrl} alt="" loading="lazy" referrerPolicy="no-referrer" /> : <Archive />}</span><div><b>{item.name}</b><p>{item.summary}</p><small>{Intl.NumberFormat().format(item.downloads)} downloads</small></div><ExternalLink /></button>)}</div>}
    </section>

    {selected && <section className="surface modpack-version-panel"><div className="section-heading"><div><h2>{tr('Pilih versi', 'Choose a version')}</h2><p>{selected.name}</p></div></div>{busy === 'versions' || versions === null ? <Skeleton lines={3} /> : versions.length === 0 ? <EmptyState icon={HardDriveDownload} title={tr('Tidak ada versi server yang kompatibel', 'No compatible server version')} description={tr('Pack ini belum menyediakan manifest Forge atau NeoForge untuk versi Minecraft yang didukung panel.', 'This pack does not provide a Forge or NeoForge manifest for a Minecraft version supported by the panel.')} /> : <div className="modpack-version-form"><label>{tr('File modpack', 'Modpack file')}<select value={chosenVersion?.fileId ?? ''} onChange={(event) => setFileId(event.target.value)}>{versions.map((item) => <option key={item.fileId} value={item.fileId}>{item.name} — {item.runtime} {item.minecraftVersion} ({item.releaseType})</option>)}</select></label>{chosenVersion && <div className="modpack-change-preview"><div><small>{tr('Server saat ini', 'Current server')}</small><b>{server.runtime} · Minecraft {server.version} · Java {server.javaVersion}</b></div><span>→</span><div><small>{tr('Setelah instalasi', 'After installation')}</small><b>{chosenVersion.runtime} · Minecraft {chosenVersion.minecraftVersion} · Java {chosenVersion.javaVersion}</b><em>{formatDate(chosenVersion.publishedAt)}</em></div></div>}<p className="form-note">{tr('Instalasi membuat backup otomatis, mempertahankan world dan alokasi, lalu mengganti runtime server. Plugin Paper tidak dimuat oleh Forge/NeoForge.', 'Installation creates an automatic backup, keeps the world and allocation, then replaces the server runtime. Paper plugins are not loaded by Forge/NeoForge.')}</p><ActionButton icon={Download} disabled={!!server.currentJob || !chosenVersion} onClick={() => { setConfirmation(''); setConfirmOpen(true); }}>{installed ? tr('Ganti modpack', 'Replace modpack') : tr('Pasang modpack', 'Install modpack')}</ActionButton></div>}</section>}

    <Modal open={confirmOpen} onClose={() => { if (busy !== 'install') setConfirmOpen(false); }} title={tr('Konfirmasi penggantian server', 'Confirm server replacement')} footer={<><ActionButton variant="secondary" disabled={busy === 'install'} onClick={() => setConfirmOpen(false)}>{tr('Batal', 'Cancel')}</ActionButton><ActionButton variant="danger" icon={CheckCircle2} loading={busy === 'install'} disabled={confirmation !== server.name} onClick={() => void install()}>{tr('Backup dan pasang', 'Back up and install')}</ActionButton></>}>
      <p>{tr('Server akan dihentikan. MyPanel membuat backup pra-perubahan, memasang modpack pilihan, memverifikasi server dapat berjalan, lalu mengembalikan status awalnya.', 'The server will stop. MyPanel creates a pre-change backup, installs the selected modpack, verifies it can run, then restores its previous running state.')}</p>
      <p><strong>{selected?.name}</strong><br />{chosenVersion?.name} · {chosenVersion?.runtime} {chosenVersion?.minecraftVersion}</p>
      <label>{tr(`Ketik “${server.name}” untuk melanjutkan`, `Type “${server.name}” to continue`)}<input value={confirmation} onChange={(event) => setConfirmation(event.target.value)} autoComplete="off" /></label>
    </Modal>
  </div>;
}
