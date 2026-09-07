import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { I18nProvider } from './i18n';
import { CreateServerForm } from './pages/Servers';

describe('Dashboard server creation', () => {
  afterEach(() => vi.restoreAllMocks());

  it('derives Java from the selected Minecraft version', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ server: {}, job: {} }), { status: 202, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    render(<I18nProvider><CreateServerForm catalog={[{ id: 'paper', name: 'Paper', java: 21, javaVersions: [21, 25], versions: [{ id: '26.2', java: 25 }, { id: '1.21.4', java: 21 }] }]} onCreated={async () => undefined} onError={() => undefined} /></I18nProvider>);

    fireEvent.change(screen.getByLabelText('Nama server'), { target: { value: 'Java 25 SMP' } });
    fireEvent.change(screen.getByLabelText('Versi Minecraft'), { target: { value: '26.2' } });
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Buat dan provision' })); });

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const request = fetchMock.mock.calls[0][1] as RequestInit;
    expect(JSON.parse(String(request.body))).toMatchObject({ name: 'Java 25 SMP', runtime: 'paper', javaVersion: 25 });
  });
});
