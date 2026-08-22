import { FitAddon } from '@xterm/addon-fit';
import { Terminal as XTerminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import { Pause, Play, RotateCcw, Send, Square } from 'lucide-react';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { websocketURL } from '../api';
import { ActionButton } from '../components/ui/ActionButton';
import { MetricChart } from '../components/ui/MetricChart';
import { StatusBadge, normalizeStatus } from '../components/ui/StatusBadge';
import { formatBytes, serverAddress } from '../format';
import { useI18n } from '../i18n';
import type { Metrics, Server } from '../types';

type History = { cpu: number[]; memory: number[]; disk: number[] };
type PendingWrite = { text: string; reset: boolean; follow: boolean };

export function Console({ server, csrfToken, busy, act }: { server: Server; csrfToken: string; busy: boolean; act: (server: Server, action: string) => Promise<void> }) {
  const { tr } = useI18n();
  const host = useRef<HTMLDivElement>(null); const socket = useRef<WebSocket | null>(null); const terminal = useRef<XTerminal | null>(null);
  const writeQueue = useRef<PendingWrite[]>([]); const writeFrame = useRef<number | null>(null);
  const [connection, setConnection] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');
  const [paused, setPaused] = useState(false); const pausedRef = useRef(false); const [result, setResult] = useState('');
  const [metrics, setMetrics] = useState<Metrics | null>(null); const [history, setHistory] = useState<History>({ cpu: [], memory: [], disk: [] });
  useEffect(() => { pausedRef.current = paused; }, [paused]);
  useEffect(() => {
    if (!host.current) return;
    const term = new XTerminal({ convertEol: true, disableStdin: true, fontFamily: 'JetBrains Mono, monospace', fontSize: 12, lineHeight: 1.45, cursorBlink: false, scrollback: 5000, theme: { background: '#0D1117', foreground: '#C9D1D9', cursor: '#3FB950', selectionBackground: '#264f36', black: '#484F58', red: '#F85149', green: '#3FB950', yellow: '#D29922', blue: '#58A6FF', magenta: '#BC8CFF', cyan: '#39C5CF', white: '#E6EDF3', brightBlack: '#6E7681', brightRed: '#FF7B72', brightGreen: '#56D364', brightYellow: '#E3B341', brightBlue: '#79C0FF', brightMagenta: '#D2A8FF', brightCyan: '#56D4DD', brightWhite: '#FFFFFF' } });
    const fit = new FitAddon(); term.loadAddon(fit); term.open(host.current); fit.fit(); terminal.current = term;
    const resize = new ResizeObserver(() => fit.fit()); resize.observe(host.current);
    const scroll = term.onScroll(() => { const isPaused = term.buffer.active.viewportY < term.buffer.active.baseY; pausedRef.current = isPaused; setPaused(isPaused); });
    return () => { if (writeFrame.current !== null) window.cancelAnimationFrame(writeFrame.current); writeQueue.current = []; scroll.dispose(); resize.disconnect(); term.dispose(); terminal.current = null; };
  }, []);
  useEffect(() => {
    let active = true; let retry: number | undefined;
    const enqueue = (write: PendingWrite) => {
      writeQueue.current.push(write);
      if (writeFrame.current !== null) return;
      writeFrame.current = window.requestAnimationFrame(() => {
        writeFrame.current = null; const term = terminal.current; if (!term) return;
        const batch = writeQueue.current.splice(0); let lastReset = -1; for (let index = batch.length - 1; index >= 0; index--) { if (batch[index].reset) { lastReset = index; break; } } const writes = lastReset >= 0 ? batch.slice(lastReset) : batch;
        if (lastReset >= 0) { term.clear(); term.write('\x1b[H'); }
        const text = writes.map((item) => item.text).join(''); const follow = writes.some((item) => item.follow);
        if (text) term.write(text, () => { if (follow && !pausedRef.current) term.scrollToBottom(); });
      });
    };
    const connect = () => {
      if (!active) return; setConnection('connecting');
      const ws = new WebSocket(websocketURL(`/api/v1/servers/${server.id}/console`)); socket.current = ws;
      ws.onopen = () => setConnection('connected');
      ws.onmessage = (event) => {
        const message = JSON.parse(event.data);
        if (message.type === 'log') enqueue({ text: renderConsoleText(String(message.logs ?? '')), reset: message.reset === true, follow: !pausedRef.current });
        if (message.type === 'status' && message.metrics) {
          const next = message.metrics as Metrics; setMetrics(next);
          setHistory((current) => ({ cpu: appendSample(current.cpu, next.cpuPercent), memory: appendSample(current.memory, next.memoryBytes), disk: appendSample(current.disk, next.diskBytes) }));
        }
        if (message.type === 'command-result') { const value = String(message.error || message.output || tr('Command selesai.', 'Command completed.')); setResult(value); enqueue({ text: `\r\n${message.error ? '\x1b[31m' : '\x1b[90m'}${sanitizeTerminalText(value)}\x1b[0m\r\n`, reset: false, follow: true }); }
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
    <header className="console-header"><div><span>{serverAddress(server.bindIp, server.port)}</span><h1>{server.name}</h1></div><StatusBadge status={normalizeStatus(server.state)} /><div className="console-actions">{running ? <><ActionButton size="sm" variant="danger" icon={Square} loading={busy} onClick={() => act(server, 'stop')}>{tr('Stop', 'Stop')}</ActionButton><ActionButton size="sm" variant="secondary" icon={RotateCcw} loading={busy} onClick={() => act(server, 'restart')}>{tr('Restart', 'Restart')}</ActionButton></> : <ActionButton size="sm" icon={Play} loading={busy} onClick={() => act(server, 'start')}>{tr('Mulai', 'Start')}</ActionButton>}</div></header>
    <div className="console-metrics">
      <MetricChart label="CPU" value={metrics?.cpuPercent ?? 0} max={server.cpu * 100} history={history.cpu} detail={`${((metrics?.cpuPercent ?? 0) / 100).toFixed(2)} / ${server.cpu} vCPU`} />
      <MetricChart label="RAM" value={metrics?.memoryBytes ?? 0} max={server.memoryMb * 1024 * 1024} history={history.memory} detail={`${formatBytes(metrics?.memoryBytes ?? 0)} / ${formatBytes(server.memoryMb * 1024 * 1024)}`} formatValue={formatBytes} />
      <MetricChart label="Disk" value={metrics?.diskBytes ?? 0} max={server.diskMb * 1024 * 1024} history={history.disk} detail={`${formatBytes(metrics?.diskBytes ?? 0)} / ${formatBytes(server.diskMb * 1024 * 1024)}`} formatValue={formatBytes} />
    </div>
    <div className="terminal-meta"><span className={`connection ${connection}`}>{connectionLabel}</span>{paused && <button onClick={() => { terminal.current?.scrollToBottom(); setPaused(false); }}><Pause />{tr('Scroll dijeda · lanjutkan', 'Scroll paused · resume')}</button>}</div>
    <div className="xterm-host" ref={host} aria-label={tr('Output console server', 'Server console output')} />
    <form className="command-prompt" onSubmit={submit}><span>$</span><input name="command" aria-label={tr('Command Minecraft', 'Minecraft command')} autoComplete="off" disabled={!running || connection !== 'connected'} placeholder={running ? 'say Hello from MyPanel' : tr('Server harus berjalan', 'Server must be running')} /><button disabled={!running || connection !== 'connected'} aria-label={tr('Kirim command', 'Send command')}><Send /></button></form>
    <span className="sr-only" aria-live="polite">{result}</span>
  </section>;
}

function appendSample(history: number[], value: number) { return [...history.slice(-29), Number.isFinite(value) ? value : 0]; }

export function renderConsoleText(logs: string) {
  const lines = logs.split(/\r?\n/); if (lines[lines.length - 1] === '') lines.pop();
  const rendered = lines.filter((line) => !/Thread RCON Client .* (?:started|shutting down)$/i.test(line)).map(renderConsoleLine).join('\r\n');
  return rendered ? rendered + (logs.endsWith('\n') ? '\r\n' : '') : '';
}

function renderConsoleLine(line: string) {
  const clean = sanitizeTerminalText(line).replace(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z\s+/, '');
  const minecraft = clean.match(/^\[([0-9]{2}:[0-9]{2}:[0-9]{2}) ([A-Z]+)\]:\s?(.*)$/);
  if (minecraft) {
    const [, time, level, message] = minecraft;
    const levelColor = /ERROR|FATAL|SEVERE/.test(level) ? '91' : /WARN/.test(level) ? '93' : /DEBUG|TRACE/.test(level) ? '90' : '96';
    const messageColor = /ERROR|FATAL|SEVERE/.test(level) ? '91' : /WARN/.test(level) ? '93' : '37';
    return `\x1b[90m[${time} \x1b[${levelColor}m${level}\x1b[90m]: \x1b[${messageColor}m${colorPluginTags(message)}\x1b[0m`;
  }
  if (/^\[(?:init|mc-image-helper)\]/i.test(clean)) return `\x1b[36m${clean}\x1b[0m`;
  if (/\b(?:ERROR|FATAL|SEVERE|Exception)\b/i.test(clean)) return `\x1b[91m${clean}\x1b[0m`;
  if (/\bWARN(?:ING)?\b/i.test(clean)) return `\x1b[93m${clean}\x1b[0m`;
  return `${clean}\x1b[0m`;
}

function colorPluginTags(message: string) {
  return message.replace(/^(\[[^\]]+\])/, '\x1b[95m$1\x1b[37m');
}

const minecraftColors: Record<string, string> = {
  '0': '30', '1': '34', '2': '32', '3': '36', '4': '31', '5': '35', '6': '33', '7': '37',
  '8': '90', '9': '94', a: '92', b: '96', c: '91', d: '95', e: '93', f: '97',
  k: '', l: '1', m: '9', n: '4', o: '3', r: '0',
};

export function sanitizeTerminalText(value: string) {
  return value
    .replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g, '')
    .replace(/(?:\x1b\[|\x9b)([0-?]*)([ -/]*)([@-~])/g, (_sequence, parameters: string, intermediate: string, final: string) => final === 'm' && intermediate === '' && /^[0-9;]*$/.test(parameters) ? `\x1b[${parameters}m` : '')
    .replace(/\x1b(?!\[[0-9;]*m)[ -/]*[@-~]/g, '')
    .replace(/[\x00-\x08\x0b\x0c\x0e-\x1a\x1c-\x1f\x7f-\x9f]/g, '')
    .replace(/§([0-9a-fk-or])/gi, (_match, code: string) => minecraftColors[code.toLowerCase()] ? `\x1b[${minecraftColors[code.toLowerCase()]}m` : '');
}
