import { describe, expect, it } from 'vitest';
import { canSendConsoleCommand, consoleTerminalTheme, renderConsoleText, renderLifecycleMessage, sanitizeTerminalText } from './Console';

describe('canSendConsoleCommand', () => {
  it('blocks commands until the server is fully running', () => {
    expect(canSendConsoleCommand('starting')).toBe(false);
    expect(canSendConsoleCommand('running')).toBe(true);
    expect(canSendConsoleCommand('offline')).toBe(false);
  });
});

describe('consoleTerminalTheme', () => {
  it('uses a light terminal surface in light mode', () => {
    const theme = consoleTerminalTheme('light');
    expect(theme.background).toBe('#FFFFFF');
    expect(theme.foreground).toBe('#24292F');
  });

  it('keeps the dark terminal palette in dark mode', () => {
    const theme = consoleTerminalTheme('dark');
    expect(theme.background).toBe('#0D1117');
    expect(theme.foreground).toBe('#C9D1D9');
  });
});

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

  it('converts legacy ampersand and RGB color codes used by plugins', () => {
    expect(sanitizeTerminalText('&a&lReady &x&5&5&f&f&5&5Mint')).toBe('\x1b[92m\x1b[1mReady \x1b[38;2;85;255;85mMint');
    expect(sanitizeTerminalText('R&D remains ordinary text')).toBe('R&D remains ordinary text');
  });

  it('converts MiniMessage named, RGB, and decoration tags', () => {
    expect(sanitizeTerminalText('<red>Error</red> <#55ff55><bold>Ready</bold> still green</#55ff55>'))
      .toBe('\x1b[91mError\x1b[0m \x1b[38;2;85;255;85m\x1b[1mReady\x1b[0m\x1b[38;2;85;255;85m still green\x1b[0m');
  });

  it('preserves plugin ANSI true color while rejecting non-color controls', () => {
    expect(sanitizeTerminalText('\x1b[38;2;120;40;220mPurple\x1b[0m\x1b[2J'))
      .toBe('\x1b[38;2;120;40;220mPurple\x1b[0m');
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

  it('does not flatten colors embedded in plugin messages', () => {
    const rendered = renderConsoleText('[06:59:00 INFO]: [Plugin] <gold>Gold</gold> &bAqua §cRed\n');
    expect(rendered).toContain('\x1b[33mGold\x1b[0m');
    expect(rendered).toContain('\x1b[96mAqua');
    expect(rendered).toContain('\x1b[91mRed');
  });
});

describe('renderLifecycleMessage', () => {
  it('renders control-plane lifecycle events in orange', () => {
    expect(renderLifecycleMessage('Restart successful.'))
      .toBe('\r\n\x1b[38;5;208m[MyPanel] Restart successful.\x1b[0m\r\n');
  });

  it('does not allow an event message to override the lifecycle color', () => {
    expect(renderLifecycleMessage('\x1b[31munsafe color\x1b[0m')).toContain('[MyPanel] unsafe color\x1b[0m');
    expect(renderLifecycleMessage('\x1b[31munsafe color\x1b[0m')).not.toContain('\x1b[31m');
  });
});
