import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../i18n';
import type { Server } from '../types';
import { FileManager } from './FileManager';

const server = { id: 'server-1', nodeId: 'node-1', ownerUserId: 'user-1', ownerUsername: 'admin', name: 'BocahSMP', runtime: 'paper', version: '1.21.4', javaVersion: 25, memoryMb: 2048, cpu: 2, diskMb: 10240, bindIp: '0.0.0.0', port: 25565, desiredState: 'offline', state: 'offline', config: {}, lastError: null, createdAt: '2026-08-21T00:00:00Z', updatedAt: '2026-08-21T00:00:00Z' } satisfies Server;

describe('FileManager', () => {
  it('renders the table and exposes unsupported operations as disabled', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ type: 'directory', path: '', entries: [{ name: 'server.properties', path: 'server.properties', type: 'file', sizeBytes: 200, modified: '2026-08-21T00:00:00Z' }] }), { status: 200, headers: { 'Content-Type': 'application/json' } })));
    render(<I18nProvider><FileManager server={server} notify={() => undefined} onError={() => undefined} /></I18nProvider>);
    expect(await screen.findByText('server.properties')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Folder Baru' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Aksi server.properties' }));
    expect(screen.getByRole('button', { name: 'Ganti nama' })).toBeDisabled();
  });

  it('opens a text file in the editor', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ type: 'directory', path: '', entries: [{ name: 'server.properties', path: 'server.properties', type: 'file', sizeBytes: 10, modified: '2026-08-21T00:00:00Z' }] }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ type: 'file', path: 'server.properties', content: btoa('motd=hello'), encoding: 'base64', sizeBytes: 10 }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    render(<I18nProvider><FileManager server={server} notify={() => undefined} onError={() => undefined} /></I18nProvider>);
    fireEvent.click(await screen.findByRole('button', { name: 'server.properties' }));
    await waitFor(() => expect(screen.getByLabelText('Isi file')).toHaveValue('motd=hello'));
  });
});
