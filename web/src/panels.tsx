import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { api, websocketURL } from './api';
import { formatBytes, formatDate } from './format';
import type { ActionResponse, Backup, FileResponse, Metrics, Schedule, Server } from './types';

type SharedProps = { server: Server; notify: (message: string) => void; onError: (value: unknown) => void };

export function OverviewPanel({ server }: { server: Server }) {
  const [metrics, setMetrics] = useState<Metrics | null>(null);
  const [unavailable, setUnavailable] = useState(false);
  useEffect(() => {
    let active = true;
    const load = () => api<Metrics>(`/api/v1/servers/${server.id}/metrics`).then((value) => { if (active) { setMetrics(value); setUnavailable(false); } }).catch(() => { if (active) setUnavailable(true); });
    load(); const interval = window.setInterval(load, 3000);
    return () => { active = false; window.clearInterval(interval); };
  }, [server.id]);
  const memoryPercent = Math.min(100, ((metrics?.memoryBytes ?? 0) / (server.memoryMb * 1024 * 1024)) * 100);
  const diskPercent = Math.min(100, ((metrics?.diskBytes ?? 0) / (server.diskMb * 1024 * 1024)) * 100);
  return <div className="overview-grid"><section className="panel identity-card"><div className="server-identity"><span className={`state-icon large ${server.state}`} /><div><p className="eyebrow">OBSERVED STATE</p><h2>{server.state}</h2><p>Desired: {server.desiredState}</p></div></div><dl><div><dt>Runtime</dt><dd>{server.runtime} {server.version} · Java {server.javaVersion}</dd></div><div><dt>Allocation</dt><dd>{server.bindIp}:{server.port}</dd></div><div><dt>Node</dt><dd>local</dd></div><div><dt>Dibuat</dt><dd>{formatDate(server.createdAt)}</dd></div></dl></section><section className="panel metrics-card"><div className="section-head"><div><p className="eyebrow">LIVE METRICS</p><h2>Resource</h2></div>{unavailable && <span className="badge error">offline</span>}</div><Metric label="CPU" value={`${(metrics?.cpuPercent ?? 0).toFixed(1)}%`} percent={Math.min(100, (metrics?.cpuPercent ?? 0) / server.cpu)} caption={`${server.cpu} vCPU limit`} /><Metric label="Memory" value={formatBytes(metrics?.memoryBytes ?? 0)} percent={memoryPercent} caption={`${server.memoryMb} MiB total`} /><Metric label="Disk" value={formatBytes(metrics?.diskBytes ?? 0)} percent={diskPercent} caption={`${server.diskMb} MiB limit`} /><div className="player-stat"><b>{metrics?.players ?? 0}</b><span>pemain terdeteksi</span></div></section></div>;
}

function Metric({ label, value, percent, caption }: { label: string; value: string; percent: number; caption: string }) {
  return <div className="metric"><div><b>{label}</b><span>{value}</span></div><div className="progress"><i style={{ width: `${percent}%` }} /></div><small>{caption}</small></div>;
}

export function ConsolePanel({ server, csrfToken }: { server: Server; csrfToken: string }) {
  const [logs, setLogs] = useState('Menghubungkan ke console…');
  const [status, setStatus] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');
  const [result, setResult] = useState('');
  const socket = useRef<WebSocket | null>(null);
  const terminal = useRef<HTMLPreElement | null>(null);
  useEffect(() => {
    let active = true; let retry: number | undefined;
    const connect = () => {
      if (!active) return;
      setStatus('connecting');
      const ws = new WebSocket(websocketURL(`/api/v1/servers/${server.id}/console`)); socket.current = ws;
      ws.onopen = () => setStatus('connected');
      ws.onmessage = (event) => {
        const message = JSON.parse(event.data);
        if (message.type === 'log') setLogs(message.logs || 'Belum ada output console.');
        if (message.type === 'command-result') setResult(message.error || message.output || 'Command selesai.');
      };
      ws.onclose = () => { if (active) { setStatus('disconnected'); retry = window.setTimeout(connect, 2500); } };
      ws.onerror = () => ws.close();
    };
    connect(); return () => { active = false; if (retry) window.clearTimeout(retry); socket.current?.close(); };
  }, [server.id]);
  useEffect(() => { if (terminal.current) terminal.current.scrollTop = terminal.current.scrollHeight; }, [logs]);
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); const form = event.currentTarget; const data = new FormData(form); const command = String(data.get('command') ?? '').trim();
    if (!command || socket.current?.readyState !== WebSocket.OPEN) return;
    socket.current.send(JSON.stringify({ type: 'command', command, csrfToken })); form.reset(); setResult('Mengirim command…');
  };
  return <section className="panel console-panel"><div className="section-head"><div><p className="eyebrow">WEBSOCKET + RCON</p><h2>Live console</h2></div><span className={`connection ${status}`}>{status}</span></div><pre ref={terminal} aria-live="polite">{logs}</pre><form className="command-bar" onSubmit={submit}><span>›</span><input name="command" aria-label="Command Minecraft" placeholder={server.state === 'running' ? 'say Halo dari MyPanel' : 'Server harus running untuk menerima command'} disabled={server.state !== 'running' || status !== 'connected'} autoComplete="off" /><button disabled={server.state !== 'running' || status !== 'connected'}>Kirim</button></form>{result && <p className="command-result">{result}</p>}</section>;
}

export function FilesPanel({ server, notify, onError }: SharedProps) {
  const [currentPath, setCurrentPath] = useState(''); const [response, setResponse] = useState<FileResponse | null>(null); const [loading, setLoading] = useState(true); const [editing, setEditing] = useState<{ path: string; content: string } | null>(null); const [busy, setBusy] = useState(false);
  const load = useCallback((path = currentPath) => { setLoading(true); api<FileResponse>(`/api/v1/servers/${server.id}/files?path=${encodeURIComponent(path)}`).then((value) => { setResponse(value); setCurrentPath(path); if (value.type === 'file') setEditing({ path: value.path, content: decodeBase64(value.content) }); else setEditing(null); }).catch(onError).finally(() => setLoading(false)); }, [currentPath, onError, server.id]);
  useEffect(() => { load(''); }, [server.id]); // eslint-disable-line react-hooks/exhaustive-deps
  const save = async () => { if (!editing) return; setBusy(true); try { await api(`/api/v1/servers/${server.id}/files`, { method: 'PUT', body: JSON.stringify({ path: editing.path, content: encodeBase64(editing.content), encoding: 'base64' }) }); notify('File disimpan.'); await load(editing.path); } catch (value) { onError(value); } finally { setBusy(false); } };
  const remove = async (path: string) => { if (!window.confirm(`Hapus ${path}?`)) return; setBusy(true); try { await api(`/api/v1/servers/${server.id}/files?path=${encodeURIComponent(path)}`, { method: 'DELETE' }); notify('File dihapus.'); await load(parentPath(path)); } catch (value) { onError(value); } finally { setBusy(false); } };
  const create = async () => { const name = window.prompt('Nama file baru (relatif terhadap direktori saat ini):'); if (!name) return; const path = [currentPath, name].filter(Boolean).join('/'); setEditing({ path, content: '' }); setResponse({ type: 'file', path, content: '', encoding: 'base64', sizeBytes: 0 }); };
  const crumbs = useMemo(() => currentPath.split('/').filter(Boolean), [currentPath]);
  return <section className="panel files-panel"><div className="section-head"><div><p className="eyebrow">SAFE DATA ROOT</p><h2>Files</h2></div><button onClick={create}>File baru</button></div><div className="breadcrumbs"><button onClick={() => load('')}>/data</button>{crumbs.map((crumb, index) => <button key={`${crumb}-${index}`} onClick={() => load(crumbs.slice(0, index + 1).join('/'))}>/ {crumb}</button>)}</div>{loading ? <p className="muted">Memuat file…</p> : editing ? <div className="editor"><div className="editor-head"><b>{editing.path}</b><div><button className="ghost" onClick={() => load(parentPath(editing.path))}>Kembali</button><button className="danger ghost" disabled={busy} onClick={() => remove(editing.path)}>Hapus</button><button disabled={busy} onClick={save}>{busy ? 'Menyimpan…' : 'Simpan'}</button></div></div><textarea value={editing.content} onChange={(event) => setEditing({ ...editing, content: event.target.value })} spellCheck={false} aria-label="Isi file" /></div> : response?.type === 'directory' ? <div className="file-list">{currentPath && <button className="file-row" onClick={() => load(parentPath(currentPath))}><span className="file-kind">↰</span><span><b>..</b><small>Direktori induk</small></span></button>}{response.entries.length === 0 ? <div className="empty">Direktori ini kosong.</div> : response.entries.map((entry) => <button key={entry.path} className="file-row" disabled={entry.type === 'symlink'} onClick={() => load(entry.path)}><span className="file-kind">{entry.type === 'directory' ? '▰' : entry.type === 'symlink' ? '↗' : '▤'}</span><span><b>{entry.name}</b><small>{entry.type} · {formatBytes(entry.sizeBytes)} · {formatDate(entry.modified)}</small></span></button>)}</div> : null}</section>;
}

export function BackupsPanel({ server, notify, onError }: SharedProps & { reloadServer: () => Promise<void> }) {
  const [items, setItems] = useState<Backup[] | null>(null); const [busy, setBusy] = useState(false);
  const load = useCallback(() => api<Backup[]>(`/api/v1/servers/${server.id}/backups`).then(setItems).catch(onError), [onError, server.id]);
  useEffect(() => { load(); const interval = window.setInterval(load, 4000); return () => window.clearInterval(interval); }, [load]);
  const create = async () => { const name = window.prompt('Nama backup:', `Backup ${new Date().toLocaleString('id-ID')}`); if (name === null) return; setBusy(true); try { await api(`/api/v1/servers/${server.id}/backups`, { method: 'POST', body: JSON.stringify({ name }) }); notify('Backup masuk ke antrean.'); await load(); } catch (value) { onError(value); } finally { setBusy(false); } };
  const restore = async (item: Backup) => { if (!window.confirm(`Restore ${item.name}? Server akan dihentikan dan world saat ini diganti.`)) return; setBusy(true); try { await api(`/api/v1/servers/${server.id}/backups/${item.id}/restore`, { method: 'POST', body: '{}' }); notify('Restore masuk ke antrean.'); } catch (value) { onError(value); } finally { setBusy(false); } };
  const remove = async (item: Backup) => { if (!window.confirm(`Hapus backup ${item.name} secara permanen?`)) return; setBusy(true); try { await api(`/api/v1/servers/${server.id}/backups/${item.id}`, { method: 'DELETE' }); notify('Backup dihapus.'); await load(); } catch (value) { onError(value); } finally { setBusy(false); } };
  return <section className="panel"><div className="section-head"><div><p className="eyebrow">CHECKSUMMED ARCHIVES</p><h2>Backup & restore</h2></div><button disabled={busy || !!server.currentJob} onClick={create}>Buat backup</button></div>{items === null ? <p className="muted">Memuat backup…</p> : items.length === 0 ? <div className="empty"><b>Belum ada backup</b><span>Buat backup sebelum mengubah mod, plugin, atau world.</span></div> : <div className="backup-list">{items.map((item) => <article key={item.id}><div><b>{item.name}</b><small>{formatDate(item.createdAt)} · {formatBytes(item.sizeBytes)}{item.checksumSha256 ? ` · SHA-256 ${item.checksumSha256.slice(0, 12)}…` : ''}</small>{item.error && <em>{item.error}</em>}</div><span className={`badge ${item.status}`}>{item.status}</span><div className="row-actions"><button className="ghost" disabled={busy || item.status !== 'ready' || !!server.currentJob} onClick={() => restore(item)}>Restore</button><button className="danger ghost" disabled={busy || !['ready', 'failed'].includes(item.status)} onClick={() => remove(item)}>Hapus</button></div></article>)}</div>}</section>;
}

export function SchedulesPanel({ server, notify, onError }: SharedProps) {
  const [items, setItems] = useState<Schedule[] | null>(null); const [busy, setBusy] = useState(false); const [action, setAction] = useState('restart');
  const load = useCallback(() => api<Schedule[]>(`/api/v1/servers/${server.id}/schedules`).then(setItems).catch(onError), [onError, server.id]);
  useEffect(() => { load(); }, [load]);
  const create = async (event: FormEvent<HTMLFormElement>) => { event.preventDefault(); setBusy(true); const form = event.currentTarget; const data = new FormData(form); try { await api(`/api/v1/servers/${server.id}/schedules`, { method: 'POST', body: JSON.stringify({ name: data.get('name'), action, intervalMinutes: Number(data.get('interval')), payload: action === 'command' ? { command: data.get('command') } : {} }) }); form.reset(); notify('Jadwal dibuat.'); await load(); } catch (value) { onError(value); } finally { setBusy(false); } };
  const remove = async (id: string) => { setBusy(true); try { await api(`/api/v1/servers/${server.id}/schedules/${id}`, { method: 'DELETE' }); notify('Jadwal dihapus.'); await load(); } catch (value) { onError(value); } finally { setBusy(false); } };
  return <div className="split-grid"><form className="panel" onSubmit={create}><p className="eyebrow">RECURRING JOB</p><h2>Jadwal baru</h2><label>Nama<input name="name" required maxLength={64} placeholder="Backup harian" /></label><label>Aksi<select value={action} onChange={(event) => setAction(event.target.value)}><option value="restart">Restart</option><option value="backup">Backup</option><option value="start">Start</option><option value="stop">Stop</option><option value="command">Command</option></select></label>{action === 'command' && <label>Command<input name="command" required placeholder="say Restart 5 menit lagi" /></label>}<label>Interval<input name="interval" type="number" min="1" max="525600" defaultValue="1440" required /><small>menit</small></label><button disabled={busy}>Simpan jadwal</button></form><section className="panel"><div className="section-head"><div><p className="eyebrow">AUTOMATION</p><h2>Jadwal aktif</h2></div></div>{items === null ? <p className="muted">Memuat…</p> : items.length === 0 ? <div className="empty">Belum ada jadwal.</div> : <div className="schedule-list">{items.map((item) => <article key={item.id}><span className="schedule-icon">↻</span><div><b>{item.name}</b><small>{item.action} setiap {item.intervalMinutes} menit · berikutnya {formatDate(item.nextRunAt)}</small></div><button className="danger ghost" disabled={busy} onClick={() => remove(item.id)}>Hapus</button></article>)}</div>}</section></div>;
}

export function SettingsPanel({ server, reload, notify, onError, remove }: SharedProps & { reload: () => Promise<void>; remove: (server: Server, purge: boolean) => Promise<void> }) {
  const config = server.config ?? {}; const [busy, setBusy] = useState(false);
  const save = async (event: FormEvent<HTMLFormElement>) => { event.preventDefault(); setBusy(true); const data = new FormData(event.currentTarget); try { await api<ActionResponse>(`/api/v1/servers/${server.id}/config`, { method: 'PUT', body: JSON.stringify({ motd: data.get('motd'), difficulty: data.get('difficulty'), gamemode: data.get('gamemode'), maxPlayers: Number(data.get('maxPlayers')), viewDistance: Number(data.get('viewDistance')), simulationDistance: Number(data.get('simulationDistance')), onlineMode: data.get('onlineMode') === 'on', whiteList: data.get('whiteList') === 'on', whiteListPlayers: data.get('whiteListPlayers') }) }); notify('Konfigurasi masuk ke antrean; container akan direkonstruksi dengan data yang sama.'); await reload(); } catch (value) { onError(value); } finally { setBusy(false); } };
  return <div className="split-grid settings-grid"><form className="panel" onSubmit={save}><p className="eyebrow">SERVER.PROPERTIES</p><h2>Konfigurasi gameplay</h2><label>MOTD<input name="motd" defaultValue={String(config.motd ?? 'A MyPanel Minecraft Server')} /></label><div className="form-row"><label>Difficulty<select name="difficulty" defaultValue={String(config.difficulty ?? 'normal')}><option>peaceful</option><option>easy</option><option>normal</option><option>hard</option></select></label><label>Gamemode<select name="gamemode" defaultValue={String(config.gamemode ?? 'survival')}><option>survival</option><option>creative</option><option>adventure</option><option>spectator</option></select></label></div><div className="form-row"><label>Max players<input name="maxPlayers" type="number" min="1" max="1000" defaultValue={Number(config.maxPlayers ?? 20)} /></label><label>View distance<input name="viewDistance" type="number" min="2" max="32" defaultValue={Number(config.viewDistance ?? 10)} /></label></div><label>Simulation distance<input name="simulationDistance" type="number" min="2" max="32" defaultValue={Number(config.simulationDistance ?? 10)} /></label><label className="checkbox"><input name="onlineMode" type="checkbox" defaultChecked={config.onlineMode !== false} /> Online mode</label><label className="checkbox"><input name="whiteList" type="checkbox" defaultChecked={config.whiteList === true} /> Aktifkan whitelist</label><label>Whitelist players<input name="whiteListPlayers" defaultValue={String(config.whiteListPlayers ?? '')} placeholder="Steve, Alex" /></label><button disabled={busy || !!server.currentJob}>{busy ? 'Menyimpan…' : 'Simpan & apply'}</button></form><section className="panel danger-zone"><p className="eyebrow">DANGER ZONE</p><h2>Hapus server</h2><p>Menghapus runtime akan membebaskan port. Anda dapat mempertahankan data untuk pemulihan manual atau menghapus seluruh world secara permanen.</p><button className="danger ghost" disabled={busy || !!server.currentJob} onClick={() => remove(server, false)}>Hapus, pertahankan data</button><button className="danger" disabled={busy || !!server.currentJob} onClick={() => remove(server, true)}>Hapus seluruh data</button></section></div>;
}

function parentPath(path: string) { const parts = path.split('/').filter(Boolean); parts.pop(); return parts.join('/'); }
function decodeBase64(value: string) { const binary = atob(value); const bytes = new Uint8Array(binary.length); for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index); return new TextDecoder().decode(bytes); }
function encodeBase64(value: string) { const bytes = new TextEncoder().encode(value); let binary = ''; for (let index = 0; index < bytes.length; index += 0x8000) binary += String.fromCharCode(...bytes.subarray(index, index + 0x8000)); return btoa(binary); }
