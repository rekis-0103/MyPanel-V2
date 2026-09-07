import { CheckSquare, Download, Edit3, File, FilePlus2, Folder, FolderPlus, MoreHorizontal, Trash2, Upload, X } from 'lucide-react';
import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from 'react';
import { api } from '../api';
import { ActionButton } from '../components/ui/ActionButton';
import { Modal } from '../components/ui/Modal';
import { EmptyState, Skeleton } from '../components/ui/States';
import { formatBytes, formatDate } from '../format';
import { useI18n } from '../i18n';
import type { FileEntry, FileResponse, Server } from '../types';

const MAX_FILE_BYTES = 10 * 1024 * 1024;
type Editor = { path: string; content: string; original: string };

export function FileManager({ server, notify, onError }: { server: Server; notify: (message: string, variant?: 'success' | 'error' | 'info' | 'warning') => void; onError: (error: unknown) => void }) {
  const { tr } = useI18n();
  const uploadInput = useRef<HTMLInputElement>(null);
  const [currentPath, setCurrentPath] = useState('');
  const [response, setResponse] = useState<FileResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [menu, setMenu] = useState<FileEntry | null>(null);
  const [editing, setEditing] = useState<Editor | null>(null);
  const [deleteTargets, setDeleteTargets] = useState<string[]>([]);
  const [createKind, setCreateKind] = useState<'file' | 'folder' | null>(null);
  const [fileName, setFileName] = useState('');
  const [rename, setRename] = useState<FileEntry | null>(null);
  const [renameName, setRenameName] = useState('');
  const [pendingPath, setPendingPath] = useState<string | null>(null);

  const load = useCallback(async (path = '') => {
    setLoading(true); setMenu(null); setSelected(new Set());
    try {
      const value = await api<FileResponse>(`/api/v1/servers/${server.id}/files?path=${encodeURIComponent(path)}`);
      setResponse(value); setCurrentPath(value.type === 'directory' ? value.path : parentPath(value.path));
      if (value.type === 'file') openEditor(value, setEditing, notify, tr); else setEditing(null);
    } catch (error) { setResponse(null); setEditing(null); onError(error); }
    finally { setLoading(false); }
  }, [notify, onError, server.id, tr]);

  useEffect(() => { load(''); }, [load]);
  useEffect(() => {
    if (!editing || editing.content === editing.original) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [editing]);

  const entries = response?.type === 'directory' ? [...response.entries].sort((a, b) => a.type === b.type ? a.name.localeCompare(b.name) : a.type === 'directory' ? -1 : 1) : [];
  const crumbs = useMemo(() => currentPath.split('/').filter(Boolean), [currentPath]);
  const navigateTo = (path: string) => editing && editing.content !== editing.original ? setPendingPath(path) : void load(path);

  const uploadFiles = async (files: FileList | File[]) => {
    const list = [...files]; if (!list.length) return; setBusy(true);
    try {
      for (const file of list) {
        if (file.size > MAX_FILE_BYTES) throw new Error(tr(`${file.name} melebihi batas 10 MiB.`, `${file.name} exceeds the 10 MiB limit.`));
        await api(`/api/v1/servers/${server.id}/files`, { method: 'PUT', body: JSON.stringify({ path: joinPath(currentPath, file.name), content: arrayBufferToBase64(await file.arrayBuffer()), encoding: 'base64' }) });
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
  const createItem = async () => {
    const safe = fileName.trim(); if (!validName(safe)) return;
    if (createKind === 'file') { setCreateKind(null); setFileName(''); setEditing({ path: joinPath(currentPath, safe), content: '', original: '' }); setResponse(null); return; }
    setBusy(true);
    try { await api(`/api/v1/servers/${server.id}/files/folders`, { method: 'POST', body: JSON.stringify({ path: joinPath(currentPath, safe) }) }); notify(tr('Folder dibuat.', 'Folder created.'), 'success'); setCreateKind(null); setFileName(''); await load(currentPath); }
    catch (error) { onError(error); } finally { setBusy(false); }
  };
  const moveItem = async () => {
    if (!rename || !validName(renameName)) return; setBusy(true);
    try { await api(`/api/v1/servers/${server.id}/files/move`, { method: 'POST', body: JSON.stringify({ from: rename.path, to: joinPath(parentPath(rename.path), renameName.trim()) }) }); notify(tr('Item diganti nama.', 'Item renamed.'), 'success'); setRename(null); await load(currentPath); }
    catch (error) { onError(error); } finally { setBusy(false); }
  };
  const download = async (entry: FileEntry) => {
    try { const value = await api<FileResponse>(`/api/v1/servers/${server.id}/files?path=${encodeURIComponent(entry.path)}`); if (value.type === 'file') saveBlob(base64ToBytes(value.content), entry.name); }
    catch (error) { onError(error); }
  };
  const toggle = (path: string) => setSelected((current) => { const next = new Set(current); if (next.has(path)) next.delete(path); else next.add(path); return next; });
  const drag = (event: DragEvent) => { event.preventDefault(); setDragging(true); };

  return <section className={`surface file-manager ${dragging ? 'dragging' : ''}`} onClick={() => setMenu(null)} onDragEnter={drag} onDragOver={drag} onDragLeave={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node)) setDragging(false); }} onDrop={(event) => { event.preventDefault(); setDragging(false); uploadFiles(event.dataTransfer.files); }}>
    {dragging && <div className="drop-overlay"><Upload /><b>{tr('Lepaskan file untuk mengunggah', 'Drop files to upload')}</b><span>{tr('Maksimum 10 MiB per file', 'Maximum 10 MiB per file')}</span></div>}
    <header className="file-toolbar"><div className="path-breadcrumb"><button onClick={() => navigateTo('')}>/data</button>{crumbs.map((crumb, index) => <button key={`${crumb}-${index}`} onClick={() => navigateTo(crumbs.slice(0, index + 1).join('/'))}>/<span>{crumb}</span></button>)}</div><div><input ref={uploadInput} type="file" multiple hidden onChange={(event) => event.target.files && uploadFiles(event.target.files)} /><ActionButton size="sm" icon={Upload} loading={busy} onClick={() => uploadInput.current?.click()}>{tr('Upload', 'Upload')}</ActionButton><ActionButton size="sm" variant="secondary" icon={FilePlus2} onClick={() => setCreateKind('file')}>{tr('File Baru', 'New File')}</ActionButton><ActionButton size="sm" variant="secondary" icon={FolderPlus} onClick={() => setCreateKind('folder')}>{tr('Folder Baru', 'New Folder')}</ActionButton></div></header>
    {selected.size > 0 && <div className="selection-bar"><CheckSquare /><span>{tr(`${selected.size} item dipilih`, `${selected.size} items selected`)}</span><ActionButton size="sm" variant="danger" icon={Trash2} onClick={() => setDeleteTargets([...selected])}>{tr('Hapus', 'Delete')}</ActionButton><button className="icon-button" aria-label={tr('Batal pilih', 'Clear selection')} onClick={() => setSelected(new Set())}><X /></button></div>}
    {editing ? <div className="file-editor"><header><div><File /><span><b>{editing.path}</b><small>{editing.content === editing.original ? tr('Tersimpan', 'Saved') : tr('Perubahan belum disimpan', 'Unsaved changes')}</small></span></div><div><ActionButton size="sm" variant="ghost" onClick={() => navigateTo(parentPath(editing.path))}>{tr('Kembali', 'Back')}</ActionButton><ActionButton size="sm" loading={busy} disabled={editing.content === editing.original} onClick={save}>{tr('Simpan', 'Save')}</ActionButton></div></header><textarea aria-label={tr('Isi file', 'File contents')} value={editing.content} onChange={(event) => setEditing({ ...editing, content: event.target.value })} spellCheck={false} /></div> : response?.type === 'file' ? <EmptyState icon={Download} title={tr('File biner', 'Binary file')} description={tr('File ini tidak aman dibuka sebagai teks. Unduh untuk melihat isinya.', 'This file is not safe to open as text. Download it to inspect its contents.')} action={{ label: tr('Download File', 'Download File'), onClick: () => saveBlob(base64ToBytes(response.content), response.path.split('/').pop() ?? 'download') }} /> : loading ? <Skeleton lines={6} /> : <FileTable entries={entries} selected={selected} menu={menu} tr={tr} onToggle={toggle} onSelectAll={() => setSelected(selected.size === entries.length ? new Set() : new Set(entries.map((entry) => entry.path)))} onOpen={navigateTo} onMenu={setMenu} onDownload={download} onRename={(entry) => { setRename(entry); setRenameName(entry.name); setMenu(null); }} onRemove={(entry) => { setDeleteTargets([entry.path]); setMenu(null); }} onUpload={() => uploadInput.current?.click()} />}
    <Modal open={deleteTargets.length > 0} onClose={() => setDeleteTargets([])} title={tr('Hapus item?', 'Delete items?')} footer={<><ActionButton variant="secondary" onClick={() => setDeleteTargets([])}>{tr('Batal', 'Cancel')}</ActionButton><ActionButton variant="danger" loading={busy} onClick={remove}>{tr('Hapus permanen', 'Delete permanently')}</ActionButton></>}><p>{tr(`${deleteTargets.length} item akan dihapus permanen dari data server.`, `${deleteTargets.length} items will be permanently deleted from server data.`)}</p></Modal>
    <Modal open={!!createKind} onClose={() => setCreateKind(null)} title={createKind === 'folder' ? tr('Folder baru', 'New folder') : tr('File baru', 'New file')} footer={<><ActionButton variant="secondary" onClick={() => setCreateKind(null)}>{tr('Batal', 'Cancel')}</ActionButton><ActionButton loading={busy} onClick={createItem} disabled={!validName(fileName)}>{createKind === 'folder' ? tr('Buat Folder', 'Create Folder') : tr('Buat File', 'Create File')}</ActionButton></>}><label>{tr('Nama', 'Name')}<input value={fileName} onChange={(event) => setFileName(event.target.value)} placeholder={createKind === 'folder' ? 'plugins' : 'server.properties'} /></label></Modal>
    <Modal open={!!rename} onClose={() => setRename(null)} title={tr('Ganti nama', 'Rename')} footer={<><ActionButton variant="secondary" onClick={() => setRename(null)}>{tr('Batal', 'Cancel')}</ActionButton><ActionButton loading={busy} onClick={moveItem} disabled={!validName(renameName) || renameName === rename?.name}>{tr('Ganti nama', 'Rename')}</ActionButton></>}><label>{tr('Nama baru', 'New name')}<input value={renameName} onChange={(event) => setRenameName(event.target.value)} /></label></Modal>
    <Modal open={pendingPath !== null} onClose={() => setPendingPath(null)} title={tr('Buang perubahan?', 'Discard changes?')} footer={<><ActionButton variant="secondary" onClick={() => setPendingPath(null)}>{tr('Lanjut mengedit', 'Keep editing')}</ActionButton><ActionButton variant="danger" onClick={() => { const path = pendingPath ?? ''; setPendingPath(null); load(path); }}>{tr('Buang perubahan', 'Discard changes')}</ActionButton></>}><p>{tr('Perubahan yang belum disimpan akan hilang.', 'Your unsaved changes will be lost.')}</p></Modal>
  </section>;
}

function FileTable({ entries, selected, menu, tr, onToggle, onSelectAll, onOpen, onMenu, onDownload, onRename, onRemove, onUpload }: { entries: FileEntry[]; selected: Set<string>; menu: FileEntry | null; tr: (id: string, en: string) => string; onToggle: (path: string) => void; onSelectAll: () => void; onOpen: (path: string) => void; onMenu: (entry: FileEntry | null) => void; onDownload: (entry: FileEntry) => void; onRename: (entry: FileEntry) => void; onRemove: (entry: FileEntry) => void; onUpload: () => void }) {
  return <div className="file-table-wrap"><table className="file-table"><thead><tr><th><input type="checkbox" aria-label={tr('Pilih semua', 'Select all')} checked={entries.length > 0 && selected.size === entries.length} onChange={onSelectAll} /></th><th>{tr('Nama', 'Name')} ↑</th><th>{tr('Ukuran', 'Size')}</th><th>{tr('Diubah', 'Modified')}</th><th><span className="sr-only">Actions</span></th></tr></thead><tbody>{entries.map((entry) => <tr key={entry.path} onContextMenu={(event) => { event.preventDefault(); event.stopPropagation(); onMenu(entry); }}><td><input type="checkbox" aria-label={tr(`Pilih ${entry.name}`, `Select ${entry.name}`)} checked={selected.has(entry.path)} onChange={() => onToggle(entry.path)} /></td><td><button className="file-name" disabled={entry.type === 'symlink'} onClick={() => onOpen(entry.path)}>{entry.type === 'directory' ? <Folder /> : <File />}<span>{entry.name}</span></button></td><td>{entry.type === 'directory' ? '—' : formatBytes(entry.sizeBytes)}</td><td>{formatDate(entry.modified)}</td><td className="file-actions"><button className="icon-button" aria-label={tr(`Aksi ${entry.name}`, `${entry.name} actions`)} onClick={(event) => { event.stopPropagation(); onMenu(menu?.path === entry.path ? null : entry); }}><MoreHorizontal /></button>{menu?.path === entry.path && <div className="context-menu" onClick={(event) => event.stopPropagation()}><button onClick={() => onOpen(entry.path)}><Edit3 />{entry.type === 'directory' ? tr('Buka', 'Open') : tr('Edit', 'Edit')}</button><button disabled={entry.type !== 'file'} onClick={() => onDownload(entry)}><Download />{tr('Download', 'Download')}</button><button onClick={() => onRename(entry)}><Edit3 />{tr('Ganti nama', 'Rename')}</button><button className="danger" onClick={() => onRemove(entry)}><Trash2 />{tr('Hapus', 'Delete')}</button></div>}</td></tr>)}</tbody></table>{entries.length === 0 && <EmptyState icon={Upload} title={tr('Folder ini kosong', 'This folder is empty')} description={tr('Unggah file konfigurasi, plugin, atau world untuk memulai.', 'Upload configuration, plugin, or world files to get started.')} action={{ label: tr('Upload File', 'Upload File'), onClick: onUpload }} />}</div>;
}

function openEditor(value: Extract<FileResponse, { type: 'file' }>, setEditing: (value: Editor | null) => void, notify: (message: string, variant?: 'success' | 'error' | 'info' | 'warning') => void, tr: (id: string, en: string) => string) {
  try { const bytes = base64ToBytes(value.content); const content = new TextDecoder('utf-8', { fatal: true }).decode(bytes); if (content.includes('\0')) throw new Error('binary'); setEditing({ path: value.path, content, original: content }); }
  catch { setEditing(null); notify(tr('File biner tidak dapat diedit sebagai teks. Gunakan Download.', 'Binary files cannot be edited as text. Use Download.'), 'warning'); }
}
function validName(name: string) { const safe = name.trim(); return !!safe && safe !== '.' && safe !== '..' && !safe.includes('/') && !safe.includes('\\'); }
function joinPath(parent: string, name: string) { return [parent, name].filter(Boolean).join('/'); }
function parentPath(path: string) { const parts = path.split('/').filter(Boolean); parts.pop(); return parts.join('/'); }
function base64ToBytes(value: string) { const binary = atob(value); return Uint8Array.from(binary, (character) => character.charCodeAt(0)); }
function arrayBufferToBase64(buffer: ArrayBuffer) { const bytes = new Uint8Array(buffer); let binary = ''; for (let index = 0; index < bytes.length; index += 0x8000) binary += String.fromCharCode(...bytes.subarray(index, index + 0x8000)); return btoa(binary); }
function encodeText(value: string) { return arrayBufferToBase64(new TextEncoder().encode(value).buffer); }
function saveBlob(bytes: Uint8Array<ArrayBuffer>, name: string) { const href = URL.createObjectURL(new Blob([bytes], { type: 'application/octet-stream' })); const anchor = document.createElement('a'); anchor.href = href; anchor.download = name; anchor.click(); window.setTimeout(() => URL.revokeObjectURL(href), 0); }
