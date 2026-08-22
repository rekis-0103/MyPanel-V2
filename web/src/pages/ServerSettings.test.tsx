import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import type { Server } from '../types';
import { ServerSettings } from './ServerSettings';

const server = { id: 'server-1', nodeId: 'node-1', name: 'BocahSMP', runtime: 'paper', version: '1.21.4', javaVersion: 25, memoryMb: 2048, cpu: 2, diskMb: 10240, bindIp: '0.0.0.0', port: 25565, desiredState: 'offline', state: 'offline', config: {}, lastError: null, createdAt: '2026-08-21T00:00:00Z', updatedAt: '2026-08-21T00:00:00Z' } satisfies Server;

describe('ServerSettings danger zone', () => {
  it('requires the exact server name before permanent deletion', () => {
    const remove = vi.fn();
    render(<I18nProvider><ServerSettings server={server} reload={async () => undefined} notify={() => undefined} onError={() => undefined} remove={remove} /></I18nProvider>);
    fireEvent.click(screen.getByRole('button', { name: 'Zona Bahaya' }));
    fireEvent.click(screen.getByRole('button', { name: 'Hapus permanen' }));
    const confirm = screen.getByRole('button', { name: 'Konfirmasi Hapus' });
    expect(confirm).toBeDisabled();
    fireEvent.change(screen.getByLabelText('Nama server'), { target: { value: 'BocahSMP' } });
    expect(confirm).toBeEnabled();
  });

  it('exposes controlled startup options without an arbitrary shell field', () => {
    render(<I18nProvider><ServerSettings server={server} reload={async () => undefined} notify={() => undefined} onError={() => undefined} remove={vi.fn()} /></I18nProvider>);
    fireEvent.click(screen.getByRole('button', { name: 'Startup' }));
    expect(screen.getByLabelText('JVM options')).toBeInTheDocument();
    expect(screen.getByLabelText('Argumen server')).toBeInTheDocument();
    expect(screen.getByLabelText('Perintah yang dikelola')).toHaveAttribute('readonly');
  });
});
