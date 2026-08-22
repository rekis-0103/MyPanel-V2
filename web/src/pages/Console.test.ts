import { describe, expect, it } from 'vitest';
import { renderConsoleText, sanitizeTerminalText } from './Console';

describe('sanitizeTerminalText', () => {
  it('removes terminal erase-line sequences without leaving CSI text behind', () => {
    expect(sanitizeTerminalText('\x1b[K[15:28:06 INFO]: Done')).toBe('[15:28:06 INFO]: Done');
  });

  it('preserves safe ANSI colors while removing cursor controls', () => {
    expect(sanitizeTerminalText('\x1b[31mERROR\x1b[0m\x1b[2J')).toBe('\x1b[31mERROR\x1b[0m');
  });

  it('removes operating system command sequences', () => {
    expect(sanitizeTerminalText('\x1b]0;Minecraft server\x07Ready')).toBe('Ready');
  });

  it('converts Minecraft color codes to terminal colors', () => {
    expect(sanitizeTerminalText('§aReady §cError')).toBe('\x1b[92mReady \x1b[91mError');
  });
});

describe('renderConsoleText', () => {
  it('filters RCON connection lifecycle noise', () => {
    const logs = '[06:09:48 INFO]: Thread RCON Client /0:0:0:0:0:0:0:1 started\n[06:09:48 INFO]: hello\n[06:09:48 INFO]: Thread RCON Client /0:0:0:0:0:0:0:1 shutting down\n';
    const rendered = renderConsoleText(logs);
    expect(rendered).not.toContain('Thread RCON Client');
    expect(rendered).toContain('\x1b[96mINFO');
    expect(rendered).toContain('hello');
  });

  it('removes Docker timestamps and colors Minecraft log levels', () => {
    const logs = '2026-08-22T06:59:03.145747110Z [06:59:03 INFO]: Done (31.886s)!\n'
      + '2026-08-22T06:59:04.000000000Z [06:59:04 WARN]: Falling behind\n'
      + '2026-08-22T06:59:05.000000000Z [06:59:05 ERROR]: Server failed\n';
    const rendered = renderConsoleText(logs);
    expect(rendered).not.toContain('2026-08-22T');
    expect(rendered).toContain('\x1b[96mINFO');
    expect(rendered).toContain('\x1b[93mWARN');
    expect(rendered).toContain('\x1b[91mERROR');
  });

  it('colors plugin tags independently from normal messages', () => {
    expect(renderConsoleText('[06:59:00 INFO]: [spark] Starting profiler\n')).toContain('\x1b[95m[spark]\x1b[37m');
  });
});
