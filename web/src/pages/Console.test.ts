import { describe, expect, it } from 'vitest';
import { sanitizeTerminalText } from './Console';

describe('sanitizeTerminalText', () => {
  it('removes terminal erase-line sequences without leaving CSI text behind', () => {
    expect(sanitizeTerminalText('\x1b[K[15:28:06 INFO]: Done')).toBe('[15:28:06 INFO]: Done');
  });

  it('removes ANSI styling sequences while preserving their content', () => {
    expect(sanitizeTerminalText('\x1b[31mERROR\x1b[0m')).toBe('ERROR');
  });

  it('removes operating system command sequences', () => {
    expect(sanitizeTerminalText('\x1b]0;Minecraft server\x07Ready')).toBe('Ready');
  });
});
