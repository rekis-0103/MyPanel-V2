import { describe, expect, it, vi } from 'vitest';
import { createUUID } from './uuid';

const uuidV4Pattern = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

describe('createUUID', () => {
  it('uses the browser native implementation when available', () => {
    const randomUUID = vi.fn(() => '11111111-1111-4111-8111-111111111111');
    expect(createUUID({ randomUUID })).toBe('11111111-1111-4111-8111-111111111111');
    expect(randomUUID).toHaveBeenCalledOnce();
  });

  it('creates a valid UUID v4 when randomUUID is unavailable over HTTP', () => {
    const getRandomValues = vi.fn((bytes: Uint8Array) => {
      bytes.fill(0xab);
      return bytes;
    });
    const uuid = createUUID({ getRandomValues });
    expect(uuid).toMatch(uuidV4Pattern);
    expect(uuid).toBe('abababab-abab-4bab-abab-abababababab');
  });
});
