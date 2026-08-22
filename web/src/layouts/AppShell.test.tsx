import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { I18nProvider } from '../i18n';
import type { Server, Session } from '../types';
import { AppShell } from './AppShell';

const server = { id: 'server-1', nodeId: 'node-1', name: 'BocahSMP', runtime: 'paper', version: '1.21.4', javaVersion: 25, memoryMb: 2048, cpu: 2, diskMb: 10240, bindIp: '0.0.0.0', port: 25565, desiredState: 'offline', state: 'offline', config: {}, lastError: null, createdAt: '2026-08-21T00:00:00Z', updatedAt: '2026-08-21T00:00:00Z' } satisfies Server;
const session = { username: 'admin', role: 'owner', csrfToken: 'token' } satisfies Session;

function renderShell(path: string) {
  return render(<I18nProvider><MemoryRouter initialEntries={[path]}><AppShell servers={[server]} session={session} onLogout={async () => undefined}><div>content</div></AppShell></MemoryRouter></I18nProvider>);
}

describe('AppShell contextual navigation', () => {
  it('shows only global navigation before a server is selected', () => {
    renderShell('/dashboard');
    expect(screen.getAllByRole('link', { name: 'Dashboard' }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole('link', { name: 'Server' }).length).toBeGreaterThan(0);
    expect(screen.queryByRole('link', { name: 'Console' })).not.toBeInTheDocument();
  });

  it('reveals server tools on a server route', () => {
    renderShell('/servers/server-1/console');
    expect(screen.getAllByRole('link', { name: 'Console' }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole('link', { name: 'File' }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole('link', { name: 'Pengaturan' }).length).toBeGreaterThan(0);
  });

  it('persists the selected color theme', () => {
    localStorage.setItem('mypanel.theme', 'dark'); renderShell('/dashboard');
    fireEvent.click(screen.getByRole('button', { name: 'Gunakan mode terang' }));
    expect(document.documentElement.dataset.theme).toBe('light');
    expect(localStorage.getItem('mypanel.theme')).toBe('light');
  });
});
