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
    expect(renderConsoleText(logs)).toBe('[06:09:48 INFO]: hello\x1b[0m\r\n');
  });
});
