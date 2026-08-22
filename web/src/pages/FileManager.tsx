import { CheckSquare, Download, Edit3, File, FilePlus2, Folder, FolderPlus, MoreHorizontal, Trash2, Upload, X } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from 'react';
import { api } from '../api';
import { ActionButton } from '../components/ui/ActionButton';
import { EmptyState, Skeleton } from '../components/ui/States';
import { Modal } from '../components/ui/Modal';
import { useI18n } from '../i18n';
import { formatBytes, formatDate } from '../format';
import type { FileEntry, FileResponse, Server } from '../types';

const MAX_FILE_BYTES = 10 * 1024 * 1024;

export function FileManager({ server, notify, onError }: { server: Server; notify: (message: string, variant?: 'success' | 'error' | 'info' | 'warning') => void; onError: (error: unknown) => void }) {
  const { tr } = useI18n(); const uploadInput = useRef<HTMLInputElement>(null);
  const [currentPath, setCurrentPath] = useState(''); const [response, setResponse] = useState<FileResponse | null>(null); const [loading, setLoading] = useState(true); const [busy, setBusy] = useState(false); const [dragging, setDragging] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set()); const [menu, setMenu] = useState<FileEntry | null>(null); const [editing, setEditing] = useState<{ path: string; content: string } | null>(null); const [deleteTargets, setDeleteTargets] = useState<string[]>([]); const [newFile, setNewFile] = useState(false); const [fileName, setFileName] = useState('');
  const load = useCallback(async (path = '') => {
    setLoading(true); setMenu(null); setSelected(new Set());
    try {
      const value = await api<FileResponse>(`/api/v1/servers/${server.id}/files?path=${encodeURIComponent(path)}`);
      setResponse(value); setCurrentPath(value.type === 'directory' ? value.path : parentPath(value.path));
      if (value.type === 'file') openEditor(value, setEditing, notify, tr); else setEditing(null);
    } catch (error) { onError(error); } finally { setLoading(false); }
  }, [notify, onError, server.id, tr]);
  useEffect(() => { load(''); }, [load]);
  const entries = response?.type === 'directory' ? [...response.entries].sort((a, b) => a.type === b.type ? a.name.localeCompare(b.name) : a.type === 'directory' ? -1 : 1) : [];
  const crumbs = useMemo(() => currentPath.split('/').filter(Boolean), [currentPath]);
  const uploadFiles = async (files: FileList | File[]) => {
    const list = [...files]; if (!list.length) return; setBusy(true);
    try {
      for (const file of list) {
        if (file.size > MAX_FILE_BYTES) throw new Error(tr(`${file.name} melebihi batas 10 MiB.`, `${file.name} exceeds the 10 MiB limit.`));
        const content = arrayBufferToBase64(await file.arrayBuffer()); const path = joinPath(currentPath, file.name);
        await api(`/api/v1/servers/${server.id}/files`, { method: 'PUT', body: JSON.stringify({ path, content, encoding: 'base64' }) });
      }
      notify(tr(`${list.length} file berhasil diunggah.`, `${list.length} files uploaded.`), 'success'); await load(currentPath);
    } catch (error) { onError(error); } finally { setBusy(false); }
  };
  const save = async () => {
    if (!editing) return; setBusy(true);
    try { await api(`/api/v1/servers/${server.id}/files`, { method: 'PUT', body: JSON.stringify({ path: editing.path, content: encodeText(editing.content), encoding: 'base64' }) }); notify(tr('File disimpan.', 'File saved.'), 'success'); await load(editing.path); }
    catch (error) { onError(error); } finally { setBusy(false); }
  };
  const remove = async () => {
    setBusy(true);
    try { for (const path of deleteTargets) await api(`/api/v1/servers/${server.id}/files?path=${encodeURIComponent(path)}`, { method: 'DELETE' }); notify(tr(`${deleteTargets.length} item dihapus.`, `${deleteTargets.length} items deleted.`), 'success'); setDeleteTargets([]); await load(currentPath); }
    catch (error) { onError(error); } finally { setBusy(false); }
  };
  const createFile = async () => {
    const safe = fileName.trim(); if (!safe || safe.includes('/') || safe.includes('\\')) return;
    setNewFile(false); setFileName(''); setEditing({ path: joinPath(currentPath, safe), content: '' }); setResponse(null);
  };
  const download = async (entry: FileEntry) => {
    try { const value = await api<FileResponse>(`/api/v1/servers/${server.id}/files?path=${encodeURIComponent(entry.path)}`); if (value.type !== 'file') return; const blob = new Blob([base64ToBytes(value.content)], { type: 'application/octet-stream' }); const href = URL.createObjectURL(blob); const anchor = document.createElement('a'); anchor.href = href; anchor.download = entry.name; anchor.click(); window.setTimeout(() => URL.revokeObjectURL(href), 0); }
    catch (error) { onError(error); }
  };
  const downloadCurrent = () => {
    if (response?.type !== 'file') return; const blob = new Blob([base64ToBytes(response.content)], { type: 'application/octet-stream' }); const href = URL.createObjectURL(blob); const anchor = document.createElement('a'); anchor.href = href; anchor.download = response.path.split('/').pop() ?? 'download'; anchor.click(); window.setTimeout(() => URL.revokeObjectURL(href), 0);
  };
  const toggle = (path: string) => setSelected((current) => { const next = new Set(current); if (next.has(path)) next.delete(path); else next.add(path); return next; });
  const drag = (event: DragEvent) => { event.preventDefault(); setDragging(true); };
  // TODO: connect New Folder and Rename once dedicated backend endpoints exist.
  return <section className={`surface file-manager ${dragging ? 'dragging' : ''}`} onDragEnter={drag} onDragOver={drag} onDragLeave={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node)) setDragging(false); }} onDrop={(event) => { event.preventDefault(); setDragging(false); uploadFiles(event.dataTransfer.files); }}>
    {dragging && <div className="drop-overlay"><Upload /><b>{tr('Lepaskan file untuk mengunggah', 'Drop files to upload')}</b><span>{tr('Maksimum 10 MiB per file', 'Maximum 10 MiB per file')}</span></div>}
    <header className="file-toolbar"><div className="path-breadcrumb"><button onClick={() => load('')}>/data</button>{crumbs.map((crumb, index) => <button key={`${crumb}-${index}`} onClick={() => load(crumbs.slice(0, index + 1).join('/'))}>/<span>{crumb}</span></button>)}</div><div><input ref={uploadInput} type="file" multiple hidden onChange={(event) => event.target.files && uploadFiles(event.target.files)} /><ActionButton size="sm" icon={Upload} loading={busy} onClick={() => uploadInput.current?.click()}>{tr('Upload', 'Upload')}</ActionButton><ActionButton size="sm" variant="secondary" icon={FilePlus2} onClick={() => setNewFile(true)}>{tr('File Baru', 'New File')}</ActionButton><ActionButton size="sm" variant="secondary" icon={FolderPlus} disabled title={tr('Memerlukan endpoint folder baru', 'Requires a create-folder endpoint')}>{tr('Folder Baru', 'New Folder')}</ActionButton></div></header>
    {selected.size > 0 && <div className="selection-bar"><CheckSquare /><span>{tr(`${selected.size} item dipilih`, `${selected.size} items selected`)}</span><ActionButton size="sm" variant="danger" icon={Trash2} onClick={() => setDeleteTargets([...selected])}>{tr('Hapus', 'Delete')}</ActionButton><button className="icon-button" onClick={() => setSelected(new Set())}><X /></button></div>}
    {editing ? <div className="file-editor"><header><div><File /><span><b>{editing.path}</b><small>{tr('Editor teks UTF-8', 'UTF-8 text editor')}</small></span></div><div><ActionButton size="sm" variant="ghost" onClick={() => load(parentPath(editing.path))}>{tr('Kembali', 'Back')}</ActionButton><ActionButton size="sm" loading={busy} onClick={save}>{tr('Simpan', 'Save')}</ActionButton></div></header><textarea aria-label={tr('Isi file', 'File contents')} value={editing.content} onChange={(event) => setEditing({ ...editing, content: event.target.value })} spellCheck={false} /></div> : response?.type === 'file' ? <EmptyState icon={Download} title={tr('File biner', 'Binary file')} description={tr('File ini tidak aman dibuka sebagai teks. Unduh untuk melihat isinya.', 'This file is not safe to open as text. Download it to inspect its contents.')} action={{ label: tr('Download File', 'Download File'), onClick: downloadCurrent }} /> : loading ? <Skeleton lines={6} /> : <div className="file-table-wrap"><table className="file-table"><thead><tr><th><input type="checkbox" aria-label={tr('Pilih semua', 'Select all')} checked={entries.length > 0 && selected.size === entries.length} onChange={() => setSelected(selected.size === entries.length ? new Set() : new Set(entries.map((entry) => entry.path)))} /></th><th>{tr('Nama', 'Name')} ↑</th><th>{tr('Ukuran', 'Size')}</th><th>{tr('Diubah', 'Modified')}</th><th><span className="sr-only">Actions</span></th></tr></thead><tbody>{entries.map((entry) => <tr key={entry.path} onContextMenu={(event) => { event.preventDefault(); setMenu(entry); }}><td><input type="checkbox" aria-label={tr(`Pilih ${entry.name}`, `Select ${entry.name}`)} checked={selected.has(entry.path)} onChange={() => toggle(entry.path)} /></td><td><button className="file-name" disabled={entry.type === 'symlink'} onClick={() => load(entry.path)}>{entry.type === 'directory' ? <Folder /> : <File />}<span>{entry.name}</span></button></td><td>{entry.type === 'directory' ? '—' : formatBytes(entry.sizeBytes)}</td><td>{formatDate(entry.modified)}</td><td className="file-actions"><button className="icon-button" aria-label={tr(`Aksi ${entry.name}`, `${entry.name} actions`)} onClick={() => setMenu(menu?.path === entry.path ? null : entry)}><MoreHorizontal /></button>{menu?.path === entry.path && <FileMenu entry={entry} tr={tr} open={() => load(entry.path)} download={() => download(entry)} remove={() => { setMenu(null); setDeleteTargets([entry.path]); }} />}</td></tr>)}</tbody></table>{entries.length === 0 && <EmptyState icon={Upload} title={tr('Folder ini kosong', 'This folder is empty')} description={tr('Unggah file konfigurasi, plugin, atau world untuk memulai.', 'Upload configuration, plugin, or world files to get started.')} action={{ label: tr('Upload File', 'Upload File'), onClick: () => uploadInput.current?.click() }} />}</div>}
    <Modal open={deleteTargets.length > 0} onClose={() => setDeleteTargets([])} title={tr('Hapus item?', 'Delete items?')} footer={<><ActionButton variant="secondary" onClick={() => setDeleteTargets([])}>{tr('Batal', 'Cancel')}</ActionButton><ActionButton variant="danger" loading={busy} onClick={remove}>{tr('Hapus permanen', 'Delete permanently')}</ActionButton></>}><p>{tr(`${deleteTargets.length} item akan dihapus permanen dari data server.`, `${deleteTargets.length} items will be permanently deleted from server data.`)}</p></Modal>
    <Modal open={newFile} onClose={() => setNewFile(false)} title={tr('File baru', 'New file')} footer={<><ActionButton variant="secondary" onClick={() => setNewFile(false)}>{tr('Batal', 'Cancel')}</ActionButton><ActionButton onClick={createFile} disabled={!fileName.trim()}>{tr('Buat File', 'Create File')}</ActionButton></>}><label>{tr('Nama file', 'File name')}<input value={fileName} onChange={(event) => setFileName(event.target.value)} placeholder="server.properties" /></label></Modal>
  </section>;
}

function FileMenu({ entry, tr, open, download, remove }: { entry: FileEntry; tr: (id: string, en: string) => string; open: () => void; download: () => void; remove: () => void }) {
  return <div className="context-menu"><button onClick={open}><Edit3 />{entry.type === 'directory' ? tr('Buka', 'Open') : tr('Edit', 'Edit')}</button><button disabled={entry.type !== 'file'} onClick={download}><Download />{tr('Download', 'Download')}</button><button disabled title={tr('Memerlukan endpoint rename', 'Requires a rename endpoint')}><Edit3 />{tr('Ganti nama', 'Rename')}</button><button className="danger" onClick={remove}><Trash2 />{tr('Hapus', 'Delete')}</button></div>;
}

function openEditor(value: Extract<FileResponse, { type: 'file' }>, setEditing: (value: { path: string; content: string } | null) => void, notify: (message: string, variant?: 'success' | 'error' | 'info' | 'warning') => void, tr: (id: string, en: string) => string) {
  try { const bytes = base64ToBytes(value.content); const content = new TextDecoder('utf-8', { fatal: true }).decode(bytes); if (content.includes('\0')) throw new Error('binary'); setEditing({ path: value.path, content }); }
  catch { setEditing(null); notify(tr('File biner tidak dapat diedit sebagai teks. Gunakan Download.', 'Binary files cannot be edited as text. Use Download.'), 'warning'); }
}
function joinPath(parent: string, name: string) { return [parent, name].filter(Boolean).join('/'); }
function parentPath(path: string) { const parts = path.split('/').filter(Boolean); parts.pop(); return parts.join('/'); }
function base64ToBytes(value: string) { const binary = atob(value); return Uint8Array.from(binary, (character) => character.charCodeAt(0)); }
function arrayBufferToBase64(buffer: ArrayBuffer) { const bytes = new Uint8Array(buffer); let binary = ''; for (let index = 0; index < bytes.length; index += 0x8000) binary += String.fromCharCode(...bytes.subarray(index, index + 0x8000)); return btoa(binary); }
function encodeText(value: string) { return arrayBufferToBase64(new TextEncoder().encode(value).buffer); }
