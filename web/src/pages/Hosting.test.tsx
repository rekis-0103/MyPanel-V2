import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { Marketplace } from './Hosting';

describe('Hosting marketplace', () => {
  afterEach(() => vi.restoreAllMocks());

  it('disables packages that exceed currently sellable capacity', async () => {
    const packages = [
      { id: 'starter', slug: 'starter', name: 'Starter', description: 'Small server', priceIdr: 29000, cpu: 1, memoryMb: 1024, diskMb: 10240, sortOrder: 10, active: true, createdAt: '', updatedAt: '' },
      { id: 'diamond', slug: 'diamond', name: 'Diamond', description: 'Large server', priceIdr: 179000, cpu: 4, memoryMb: 8192, diskMb: 81920, sortOrder: 40, active: true, createdAt: '', updatedAt: '' },
    ];
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve(new Response(JSON.stringify(path.endsWith('/packages') ? packages : {
      available: { cpu: 2, memoryMb: 4096, diskMb: 50000, ports: 5 },
      packageAvailability: { starter: true, diamond: false },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }))));

    render(<I18nProvider><Marketplace catalog={[{ id: 'paper', name: 'Paper', java: 21, javaVersions: [21, 25] }]} reloadServers={async () => undefined} notify={() => undefined} onError={() => undefined} /></I18nProvider>);

    expect(await screen.findByRole('button', { name: 'Pilih paket' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Kapasitas tidak cukup' })).toBeDisabled();
  });
});
