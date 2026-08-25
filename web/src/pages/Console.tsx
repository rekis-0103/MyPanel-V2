import { FitAddon } from '@xterm/addon-fit';
import { Terminal as XTerminal, type ITheme } from '@xterm/xterm';
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
type LifecycleEntry = { message?: unknown; createdAt?: unknown };
type TimedConsoleEntry = { at: number; order: number; text: string };

export function Console({ server, csrfToken, busy, act }: { server: Server; csrfToken: string; busy: boolean; act: (server: Server, action: string) => Promise<void> }) {
  const { tr } = useI18n();
  const host = useRef<HTMLDivElement>(null); const socket = useRef<WebSocket | null>(null); const terminal = useRef<XTerminal | null>(null);
  const writeQueue = useRef<PendingWrite[]>([]); const writeBusy = useRef(false);
  const [connection, setConnection] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');
  const [paused, setPaused] = useState(false); const pausedRef = useRef(false); const [result, setResult] = useState('');
  const [metrics, setMetrics] = useState<Metrics | null>(null); const [history, setHistory] = useState<History>({ cpu: [], memory: [], disk: [] });
  const [runtimeState, setRuntimeState] = useState(server.state);
  useEffect(() => { pausedRef.current = paused; }, [paused]);
  useEffect(() => { setRuntimeState(server.state); }, [server.state]);
  useEffect(() => {
    if (!host.current) return;
    const term = new XTerminal({ convertEol: true, disableStdin: true, fontFamily: 'JetBrains Mono, monospace', fontSize: 12, lineHeight: 1.45, cursorBlink: false, scrollback: 5000, theme: consoleTerminalTheme(currentTheme()) });
    const fit = new FitAddon(); term.loadAddon(fit); term.open(host.current); fit.fit(); terminal.current = term;
    const resize = new ResizeObserver(() => fit.fit()); resize.observe(host.current);
    const themeObserver = new MutationObserver(() => { term.options.theme = consoleTerminalTheme(currentTheme()); });
    themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    const scroll = term.onScroll(() => { const isPaused = term.buffer.active.viewportY < term.buffer.active.baseY; pausedRef.current = isPaused; setPaused(isPaused); });
    return () => { writeQueue.current = []; writeBusy.current = false; themeObserver.disconnect(); scroll.dispose(); resize.disconnect(); term.dispose(); terminal.current = null; };
  }, []);
  useEffect(() => {
    let active = true; let retry: number | undefined; let streamTimer: number | undefined; let streamOrder = 0; let streamBuffer: TimedConsoleEntry[] = [];
    const flushWrites = () => {
      const term = terminal.current;
      if (!active || !term || writeBusy.current || writeQueue.current.length === 0) return;
      const batch = writeQueue.current.splice(0); let lastReset = -1; for (let index = batch.length - 1; index >= 0; index--) { if (batch[index].reset) { lastReset = index; break; } } const writes = lastReset >= 0 ? batch.slice(lastReset) : batch;
      if (lastReset >= 0) term.clear();
      const text = (lastReset >= 0 ? '\x1b[H' : '') + writes.map((item) => item.text).join(''); const follow = writes.some((item) => item.follow);
      if (!text) { queueMicrotask(flushWrites); return; }
      writeBusy.current = true;
      term.write(text, () => {
        writeBusy.current = false;
        if (follow && !pausedRef.current) term.scrollToBottom();
        queueMicrotask(flushWrites);
      });
    };
    const enqueue = (write: PendingWrite) => {
      writeQueue.current.push(write);
      flushWrites();
    };
    const flushStream = () => {
      streamTimer = undefined;
      const entries = streamBuffer.sort((left, right) => left.at - right.at || left.order - right.order);
      streamBuffer = [];
      enqueue({ text: entries.map((entry) => entry.text).join(''), reset: false, follow: !pausedRef.current });
    };
    const enqueueStream = (logs: string, lifecycle: LifecycleEntry[]) => {
      const entries = consoleStreamEntries(logs, lifecycle, Date.now(), streamOrder);
      streamBuffer.push(...entries);
      streamOrder += entries.length;
      if (streamTimer === undefined) streamTimer = window.setTimeout(flushStream, 120);
    };
    const connect = () => {
      if (!active) return; setConnection('connecting');
      const ws = new WebSocket(websocketURL(`/api/v1/servers/${server.id}/console`)); socket.current = ws;
      ws.onopen = () => setConnection('connected');
      ws.onmessage = (event) => {
        const message = JSON.parse(event.data);
        if (message.type === 'history') {
          if (streamTimer !== undefined) window.clearTimeout(streamTimer);
          streamTimer = undefined; streamBuffer = [];
          enqueue({ text: renderConsoleHistory(String(message.logs ?? ''), Array.isArray(message.lifecycle) ? message.lifecycle : []), reset: true, follow: !pausedRef.current });
        }
        if (message.type === 'log') enqueueStream(String(message.logs ?? ''), []);
        if (message.type === 'lifecycle') {
          enqueueStream('', [{ message: message.message, createdAt: message.createdAt }]);
        }
        if (message.type === 'status') {
          if (typeof message.state === 'string') setRuntimeState(message.state as Server['state']);
          if (message.metrics) {
            const next = message.metrics as Metrics; setMetrics(next);
            setHistory((current) => ({ cpu: appendSample(current.cpu, next.cpuPercent), memory: appendSample(current.memory, next.memoryBytes), disk: appendSample(current.disk, next.diskBytes) }));
          }
        }
        if (message.type === 'command-result') { const value = String(message.error || message.output || tr('Command selesai.', 'Command completed.')); setResult(value); enqueue({ text: `\r\n${message.error ? '\x1b[31m' : '\x1b[90m'}${sanitizeTerminalText(value)}\x1b[0m\r\n`, reset: false, follow: true }); }
      };
      ws.onclose = () => { if (active) { setConnection('disconnected'); retry = window.setTimeout(connect, 2500); } };
      ws.onerror = () => ws.close();
    };
    connect(); return () => { active = false; if (retry) window.clearTimeout(retry); if (streamTimer !== undefined) window.clearTimeout(streamTimer); streamBuffer = []; writeQueue.current = []; socket.current?.close(); };
  }, [server.id, tr]);
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); const form = event.currentTarget; const command = String(new FormData(form).get('command') ?? '').trim();
    if (!command || socket.current?.readyState !== WebSocket.OPEN) return;
    socket.current.send(JSON.stringify({ type: 'command', command, csrfToken })); form.reset(); setResult(tr('Mengirim command…', 'Sending command…'));
  };
  const running = canSendConsoleCommand(runtimeState); const starting = runtimeState === 'starting';
  const connectionLabel = { connecting: tr('menghubungkan', 'connecting'), connected: tr('terhubung', 'connected'), disconnected: tr('terputus', 'disconnected') }[connection];
  return <section className="surface console-page">
    <header className="console-header"><div><span>{serverAddress(server.bindIp, server.port)}</span><h1>{server.name}</h1></div><StatusBadge status={normalizeStatus(runtimeState)} /><div className="console-actions">{running || starting ? <><ActionButton size="sm" variant="danger" icon={Square} loading={busy} onClick={() => act(server, 'stop')}>{tr('Stop', 'Stop')}</ActionButton>{running && <ActionButton size="sm" variant="secondary" icon={RotateCcw} loading={busy} onClick={() => act(server, 'restart')}>{tr('Restart', 'Restart')}</ActionButton>}</> : <ActionButton size="sm" icon={Play} loading={busy} onClick={() => act(server, 'start')}>{tr('Mulai', 'Start')}</ActionButton>}</div></header>
    <div className="console-metrics">
      <MetricChart label="CPU" value={metrics?.cpuPercent ?? 0} max={server.cpu * 100} history={history.cpu} detail={`${((metrics?.cpuPercent ?? 0) / 100).toFixed(2)} / ${server.cpu} vCPU`} />
      <MetricChart label="RAM" value={metrics?.memoryBytes ?? 0} max={server.memoryMb * 1024 * 1024} history={history.memory} detail={`${formatBytes(metrics?.memoryBytes ?? 0)} / ${formatBytes(server.memoryMb * 1024 * 1024)}`} formatValue={formatBytes} />
      <MetricChart label="Disk" value={metrics?.diskBytes ?? 0} max={server.diskMb * 1024 * 1024} history={history.disk} detail={`${formatBytes(metrics?.diskBytes ?? 0)} / ${formatBytes(server.diskMb * 1024 * 1024)}`} formatValue={formatBytes} />
    </div>
    <div className="terminal-meta"><span className={`connection ${connection}`}>{connectionLabel}</span>{paused && <button onClick={() => { terminal.current?.scrollToBottom(); setPaused(false); }}><Pause />{tr('Scroll dijeda · lanjutkan', 'Scroll paused · resume')}</button>}</div>
    <div className="xterm-host" ref={host} aria-label={tr('Output console server', 'Server console output')} />
    <form className="command-prompt" onSubmit={submit}><span>$</span><input name="command" aria-label={tr('Command Minecraft', 'Minecraft command')} autoComplete="off" disabled={!running || connection !== 'connected'} placeholder={running ? 'say Hello from MyPanel' : starting ? tr('Tunggu sampai server siap', 'Wait until the server is ready') : tr('Server harus berjalan', 'Server must be running')} /><button disabled={!running || connection !== 'connected'} aria-label={tr('Kirim command', 'Send command')}><Send /></button></form>
    <span className="sr-only" aria-live="polite">{result}</span>
  </section>;
}

type ConsoleTheme = 'dark' | 'light';

export function canSendConsoleCommand(state: Server['state']) { return state === 'running'; }

function currentTheme(): ConsoleTheme { return document.documentElement.dataset.theme === 'light' ? 'light' : 'dark'; }

export function consoleTerminalTheme(theme: ConsoleTheme): ITheme {
  if (theme === 'light') return { background: '#FFFFFF', foreground: '#24292F', cursor: '#1A7F37', selectionBackground: '#B6D7FF', black: '#24292F', red: '#CF222E', green: '#1A7F37', yellow: '#9A6700', blue: '#0969DA', magenta: '#8250DF', cyan: '#1B7C83', white: '#57606A', brightBlack: '#6E7781', brightRed: '#A40E26', brightGreen: '#116329', brightYellow: '#7D4E00', brightBlue: '#0550AE', brightMagenta: '#6639BA', brightCyan: '#0A6C74', brightWhite: '#1F2328' };
  return { background: '#0D1117', foreground: '#C9D1D9', cursor: '#3FB950', selectionBackground: '#264F36', black: '#484F58', red: '#F85149', green: '#3FB950', yellow: '#D29922', blue: '#58A6FF', magenta: '#BC8CFF', cyan: '#39C5CF', white: '#E6EDF3', brightBlack: '#6E7681', brightRed: '#FF7B72', brightGreen: '#56D364', brightYellow: '#E3B341', brightBlue: '#79C0FF', brightMagenta: '#D2A8FF', brightCyan: '#56D4DD', brightWhite: '#FFFFFF' };
}

function appendSample(history: number[], value: number) { return [...history.slice(-29), Number.isFinite(value) ? value : 0]; }

export function renderConsoleText(logs: string) {
  const lines = logs.split(/\r?\n/); if (lines[lines.length - 1] === '') lines.pop();
  const rendered = lines.filter((line) => !/Thread RCON Client .* (?:started|shutting down)$/i.test(line)).map(renderConsoleLine).join('\r\n');
  return rendered ? rendered + (logs.endsWith('\n') ? '\r\n' : '') : '';
}

export function renderLifecycleMessage(message: string) {
  const plain = sanitizeTerminalText(message).replace(/\x1b\[[0-9;]*m/g, '').trim();
  return plain ? `\x1b[38;5;208m[MyPanel] ${plain}\x1b[0m\r\n` : '';
}

export function renderConsoleHistory(logs: string, lifecycle: LifecycleEntry[]) {
  return consoleStreamEntries(logs, lifecycle, 0, 0)
    .sort((left, right) => left.at - right.at || left.order - right.order)
    .map((entry) => entry.text).join('');
}

function consoleStreamEntries(logs: string, lifecycle: LifecycleEntry[], fallbackAt: number, orderStart: number) {
  const entries: TimedConsoleEntry[] = []; let order = orderStart;
  const lines = logs.split(/\r?\n/); if (lines[lines.length - 1] === '') lines.pop();
  for (const line of lines) {
    const timestamp = line.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s/);
    const parsed = timestamp ? Date.parse(timestamp[1]) : Number.NaN;
    entries.push({ at: Number.isFinite(parsed) ? parsed : fallbackAt, order: order++, text: renderConsoleText(line + '\n') });
  }
  for (const event of lifecycle) {
    const parsed = Date.parse(String(event.createdAt ?? ''));
    const text = renderLifecycleMessage(String(event.message ?? ''));
    if (text) entries.push({ at: Number.isFinite(parsed) ? parsed : fallbackAt, order: order++, text });
  }
  return entries;
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

const miniMessageColors: Record<string, string> = {
  black: '30', dark_blue: '34', dark_green: '32', dark_aqua: '36', dark_red: '31', dark_purple: '35', gold: '33', gray: '37', grey: '37',
  dark_gray: '90', dark_grey: '90', blue: '94', green: '92', aqua: '96', red: '91', light_purple: '95', yellow: '93', white: '97',
};

const miniMessageDecorations: Record<string, string> = { bold: '1', italic: '3', underlined: '4', underline: '4', strikethrough: '9', reset: '0' };

function trueColor(hex: string) {
  const value = hex.replace('#', '');
  const red = Number.parseInt(value.slice(0, 2), 16); const green = Number.parseInt(value.slice(2, 4), 16); const blue = Number.parseInt(value.slice(4, 6), 16);
  return `\x1b[38;2;${red};${green};${blue}m`;
}

export function translatePluginFormatting(value: string) {
  const marker = '(?:\\u00c2?\\u00a7|&)';
  const legacyHex = new RegExp(`${marker}x((?:${marker}[0-9a-f]){6})`, 'gi');
  const miniMessageStack: Array<{ name: string; code: string }> = [];
  return value
    .replace(legacyHex, (_match, sequence: string) => trueColor((sequence.match(/[0-9a-f]/gi) ?? []).join('')))
    .replace(/(?:\u00c2?\u00a7)([0-9a-fk-or])/gi, (_match, code: string) => minecraftColors[code.toLowerCase()] ? `\x1b[${minecraftColors[code.toLowerCase()]}m` : '')
    .replace(/(^|[\s:;,([{])((?:&[0-9a-fk-or])+)(?=\S)/gi, (_match, prefix: string, sequence: string) => `${prefix}${sequence.replace(/&([0-9a-fk-or])/gi, (_code: string, code: string) => minecraftColors[code.toLowerCase()] ? `\x1b[${minecraftColors[code.toLowerCase()]}m` : '')}`)
    .replace(/<(\/)?([a-z_]+|#[0-9a-f]{6})>/gi, (tag, closing: string | undefined, name: string) => {
      const normalized = name.toLowerCase();
      if (closing) {
        const index = miniMessageStack.map((item) => item.name).lastIndexOf(normalized);
        if (index < 0) return tag;
        miniMessageStack.splice(index, 1);
        return `\x1b[0m${miniMessageStack.map((item) => `\x1b[${item.code}m`).join('')}`;
      }
      if (normalized === 'reset') { miniMessageStack.length = 0; return '\x1b[0m'; }
      if (/^#[0-9a-f]{6}$/.test(normalized)) {
        const value = normalized.replace('#', '');
        const code = `38;2;${Number.parseInt(value.slice(0, 2), 16)};${Number.parseInt(value.slice(2, 4), 16)};${Number.parseInt(value.slice(4, 6), 16)}`;
        miniMessageStack.push({ name: normalized, code }); return `\x1b[${code}m`;
      }
      const code = miniMessageColors[normalized] ?? miniMessageDecorations[normalized];
      if (!code) return tag;
      miniMessageStack.push({ name: normalized, code }); return `\x1b[${code}m`;
    });
}

export function sanitizeTerminalText(value: string) {
  return translatePluginFormatting(value)
    .replace(/\x1b\][^\x07]*(?:\x07|\x1b\\)/g, '')
    .replace(/(?:\x1b\[|\x9b)([0-?]*)([ -/]*)([@-~])/g, (_sequence, parameters: string, intermediate: string, final: string) => final === 'm' && intermediate === '' && /^[0-9;]*$/.test(parameters) ? `\x1b[${parameters}m` : '')
    .replace(/\x1b(?!\[[0-9;]*m)[ -/]*[@-~]/g, '')
    .replace(/[\x00-\x08\x0b\x0c\x0e-\x1a\x1c-\x1f\x7f-\x9f]/g, '');
}
