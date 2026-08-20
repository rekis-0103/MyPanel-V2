import { describe, expect, it } from 'vitest';
import { formatBytes } from './format';

describe('formatBytes', () => {
  it('formats empty and allocated values', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(2 * 1024 * 1024)).toBe('2.0 MiB');
  });
});
