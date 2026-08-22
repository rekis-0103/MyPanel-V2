import { describe, expect, it } from 'vitest';
import { formatBytes, serverAddress } from './format';

describe('formatBytes', () => {
  it('formats empty and allocated values', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(2 * 1024 * 1024)).toBe('2.0 MiB');
  });
});

describe('serverAddress', () => {
  it('advertises the panel host when Docker listens on every interface', () => {
    expect(serverAddress('0.0.0.0', 25565, '192.168.56.101')).toBe('192.168.56.101:25565');
    expect(serverAddress('10.0.0.8', 25566, '192.168.56.101')).toBe('10.0.0.8:25566');
  });
});
