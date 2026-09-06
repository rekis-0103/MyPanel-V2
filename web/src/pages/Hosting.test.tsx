import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import { AdminPackages, Marketplace } from './Hosting';

const presentation = { themeColor: '#3FB950', icon: 'grass' as const, isPopular: false, isRecommended: false };

describe('Hosting marketplace', () => {
  afterEach(() => vi.restoreAllMocks());

  it('disables packages that exceed currently sellable capacity', async () => {
    const packages = [
      { id: 'starter', slug: 'starter', name: 'Starter', description: 'Small server', priceIdr: 29000, cpu: 1, memoryMb: 1024, diskMb: 10240, sortOrder: 10, active: true, ...presentation, createdAt: '', updatedAt: '' },
      { id: 'diamond', slug: 'diamond', name: 'Diamond', description: 'Large server', priceIdr: 179000, cpu: 4, memoryMb: 8192, diskMb: 81920, sortOrder: 40, active: true, ...presentation, icon: 'diamond' as const, themeColor: '#58C7DF', createdAt: '', updatedAt: '' },
    ];
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve(new Response(JSON.stringify(path.endsWith('/packages') ? packages : {
      available: { cpu: 2, memoryMb: 4096, diskMb: 50000, ports: 5 },
      packageAvailability: { starter: true, diamond: false },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }))));

    render(<I18nProvider><Marketplace catalog={[{ id: 'paper', name: 'Paper', java: 21, javaVersions: [21, 25] }]} reloadServers={async () => undefined} notify={() => undefined} onError={() => undefined} /></I18nProvider>);

    expect(await screen.findByRole('button', { name: 'Pilih paket' })).toBeEnabled();
    expect(screen.getByRole('button', { name: 'Kapasitas tidak cukup' })).toBeDisabled();
  });

  it('derives Java automatically when the Minecraft version changes', async () => {
    const packages = [{ id: 'starter', slug: 'iron', name: 'Iron', description: 'Small server', priceIdr: 59000, cpu: 1, memoryMb: 2048, diskMb: 20480, sortOrder: 10, active: true, ...presentation, icon: 'anvil' as const, themeColor: '#AEB7C2', createdAt: '', updatedAt: '' }];
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve(new Response(JSON.stringify(path.endsWith('/packages') ? packages : {
      available: { cpu: 4, memoryMb: 8192, diskMb: 100000, ports: 10 },
      packageAvailability: { starter: true },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }))));

    render(<I18nProvider><Marketplace catalog={[{ id: 'paper', name: 'Paper', java: 21, javaVersions: [21, 25], icon: 'feather', versions: [{ id: '26.2', java: 25 }, { id: '1.21.4', java: 21 }] }]} reloadServers={async () => undefined} notify={() => undefined} onError={() => undefined} /></I18nProvider>);

    fireEvent.click(await screen.findByRole('button', { name: 'Pilih paket' }));
    expect(screen.getByText('Java 25')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Versi Minecraft'), { target: { value: '1.21.4' } });
    expect(screen.getByText('Java 21')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Paper' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('shows promotional package markers', async () => {
    const packages = [{ id: 'gold', slug: 'gold', name: 'Gold', description: 'Popular server', priceIdr: 99000, cpu: 2, memoryMb: 4096, diskMb: 40960, sortOrder: 30, active: true, ...presentation, icon: 'gold' as const, themeColor: '#E3B341', isPopular: true, isRecommended: true, createdAt: '', updatedAt: '' }];
    vi.stubGlobal('fetch', vi.fn((path: string) => Promise.resolve(new Response(JSON.stringify(path.endsWith('/packages') ? packages : { available: { cpu: 4, memoryMb: 8192, diskMb: 100000, ports: 10 }, packageAvailability: { gold: true } }), { status: 200, headers: { 'Content-Type': 'application/json' } }))));
    render(<I18nProvider><Marketplace catalog={[]} reloadServers={async () => undefined} notify={() => undefined} onError={() => undefined} /></I18nProvider>);
    expect(await screen.findByText('Paling laris')).toBeInTheDocument();
    expect(screen.getByText('Rekomendasi')).toBeInTheDocument();
  });
});

describe('Hosting package administration', () => {
  afterEach(() => vi.restoreAllMocks());

  it('edits the package color, logo, and marketplace markers', async () => {
    const item = { id: '10000000-0000-0000-0000-000000000002', slug: 'iron', name: 'Iron', description: 'Small server', priceIdr: 59000, cpu: 1, memoryMb: 2048, diskMb: 20480, sortOrder: 20, active: true, ...presentation, icon: 'anvil' as const, themeColor: '#AEB7C2', isPopular: true, createdAt: '', updatedAt: '' };
    const requestBodies: unknown[] = [];
    vi.stubGlobal('fetch', vi.fn((_path: string, init?: RequestInit) => {
      if (init?.method === 'PUT') requestBodies.push(JSON.parse(String(init.body)));
      return Promise.resolve(new Response(JSON.stringify(init?.method === 'PUT' ? item : [item]), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    }));

    render(<I18nProvider><AdminPackages notify={() => undefined} onError={() => undefined} /></I18nProvider>);
    fireEvent.click(await screen.findByRole('button', { name: 'Edit' }));
    expect(screen.getByRole('button', { name: 'Anvil' })).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(screen.getByRole('button', { name: 'Diamond' }));
    fireEvent.change(screen.getByLabelText('Pilih warna tema'), { target: { value: '#58c7df' } });
    fireEvent.click(screen.getByRole('checkbox', { name: 'Rekomendasi' }));
    fireEvent.click(screen.getByRole('button', { name: 'Simpan paket' }));

    await waitFor(() => expect(requestBodies).toHaveLength(1));
    expect(requestBodies[0]).toMatchObject({ themeColor: '#58C7DF', icon: 'diamond', isPopular: true, isRecommended: true });
  });
});
