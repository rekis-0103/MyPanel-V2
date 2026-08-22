import { FitAddon } from '@xterm/addon-fit';
import { Terminal as XTerminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import { Pause, Play, RotateCcw, Send, Square } from 'lucide-react';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { websocketURL } from '../api';
import { ActionButton } from '../components/ui/ActionButton';
import { StatusBadge, normalizeStatus } from '../components/ui/StatusBadge';
import { useI18n } from '../i18n';
import type { Server } from '../types';

export function Console({ server, csrfToken, busy, act }: { server: Server; csrfToken: string; busy: boolean; act: (server: Server, action: string) => Promise<void> }) {
  const { tr } = useI18n(); const host = useRef<HTMLDivElement>(null); const socket = useRef<WebSocket | null>(null); const terminal = useRef<XTerminal | null>(null);
  const [connection, setConnection] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');
  const [paused, setPaused] = useState(false); const pausedRef = useRef(false); const [result, setResult] = useState('');
  useEffect(() => { pausedRef.current = paused; }, [paused]);
  useEffect(() => {
    if (!host.current) return;
    const term = new XTerminal({ convertEol: true, disableStdin: true, fontFamily: 'JetBrains Mono, monospace', fontSize: 12, lineHeight: 1.45, cursorBlink: false, theme: { background: '#0D1117', foreground: '#3FB950', cursor: '#3FB950', selectionBackground: '#264f36' } });
    const fit = new FitAddon(); term.loadAddon(fit); term.open(host.current); fit.fit(); terminal.current = term;
    const resize = new ResizeObserver(() => fit.fit()); resize.observe(host.current);
    const scroll = term.onScroll(() => { const isPaused = term.buffer.active.viewportY < term.buffer.active.baseY; pausedRef.current = isPaused; setPaused(isPaused); });
    return () => { scroll.dispose(); resize.disconnect(); term.dispose(); terminal.current = null; };
  }, []);
  useEffect(() => {
    let active = true; let retry: number | undefined;
    const connect = () => {
      if (!active) return; setConnection('connecting');
      const ws = new WebSocket(websocketURL(`/api/v1/servers/${server.id}/console`)); socket.current = ws;
      ws.onopen = () => setConnection('connected');
      ws.onmessage = (event) => {
        const message = JSON.parse(event.data);
        if (message.type === 'log') renderSnapshot(terminal.current, message.logs || tr('Belum ada output console.', 'No console output yet.'), !pausedRef.current);
        if (message.type === 'command-result') { const value = String(message.error || message.output || tr('Command selesai.', 'Command completed.')); setResult(value); terminal.current?.writeln(`\r\n${message.error ? '\x1b[31m' : '\x1b[90m'}${sanitizeTerminalText(value)}\x1b[0m`); }
      };
      ws.onclose = () => { if (active) { setConnection('disconnected'); retry = window.setTimeout(connect, 2500); } };
      ws.onerror = () => ws.close();
    };
    connect(); return () => { active = false; if (retry) window.clearTimeout(retry); socket.current?.close(); };
  }, [server.id, tr]);
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); const form = event.currentTarget; const command = String(new FormData(form).get('command') ?? '').trim();
    if (!command || socket.current?.readyState !== WebSocket.OPEN) return;
    socket.current.send(JSON.stringify({ type: 'command', command, csrfToken })); form.reset(); setResult(tr('Mengirim command…', 'Sending command…'));
  };
  const running = server.state === 'running';
  const connectionLabel = { connecting: tr('menghubungkan', 'connecting'), connected: tr('terhubung', 'connected'), disconnected: tr('terputus', 'disconnected') }[connection];
  return <section className="surface console-page">
    <header className="console-header"><div><span>{tr('Server', 'Server')}</span><h1>{server.name}</h1></div><StatusBadge status={normalizeStatus(server.state)} /><div className="console-actions">{running ? <><ActionButton size="sm" variant="danger" icon={Square} loading={busy} onClick={() => act(server, 'stop')}>{tr('Stop', 'Stop')}</ActionButton><ActionButton size="sm" variant="secondary" icon={RotateCcw} loading={busy} onClick={() => act(server, 'restart')}>{tr('Restart', 'Restart')}</ActionButton></> : <ActionButton size="sm" icon={Play} loading={busy} onClick={() => act(server, 'start')}>{tr('Mulai', 'Start')}</ActionButton>}</div></header>
    <div className="terminal-meta"><span className={`connection ${connection}`}>{connectionLabel}</span>{paused && <button onClick={() => { terminal.current?.scrollToBottom(); setPaused(false); }}><Pause />{tr('Scroll dijeda · lanjutkan', 'Scroll paused · resume')}</button>}</div>
    <div className="xterm-host" ref={host} aria-label={tr('Output console server', 'Server console output')} />
    <form className="command-prompt" onSubmit={submit}><span>$</span><input name="command" aria-label={tr('Command Minecraft', 'Minecraft command')} autoComplete="off" disabled={!running || connection !== 'connected'} placeholder={running ? 'say Hello from MyPanel' : tr('Server harus berjalan', 'Server must be running')} /><button disabled={!running || connection !== 'connected'} aria-label={tr('Kirim command', 'Send command')}><Send /></button></form>
    <span className="sr-only" aria-live="polite">{result}</span>
  </section>;
}

function renderSnapshot(term: XTerminal | null, logs: string, follow: boolean) {
  if (!term) return; term.clear(); term.write('\x1b[H');
  const rendered = logs.split(/\r?\n/).map((line) => {
    const color = /\b(error|fatal|exception|severe)\b/i.test(line) ? '\x1b[31m' : /\b(info|system|warn)\b/i.test(line) ? '\x1b[90m' : '\x1b[32m';
    return `${color}${sanitizeTerminalText(line)}\x1b[0m`;
  }).join('\r\n');
  term.write(rendered); if (follow) term.scrollToBottom();
}

function sanitizeTerminalText(value: string) { return value.replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f-\x9f]/g, '').replace(/\x1b/g, ''); }
