import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Dashboard } from './App';

describe('Dashboard server creation', () => {
  afterEach(() => vi.restoreAllMocks());

  it('submits the selected Java version', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ server: {}, job: {} }), { status: 202, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    render(<Dashboard servers={[]} catalog={[{ id: 'paper', name: 'Paper', java: 21, javaVersions: [21, 25] }]} busy={false} select={() => undefined} onCreated={async () => undefined} onError={() => undefined} setBusy={() => undefined} />);

    fireEvent.change(screen.getByLabelText('Nama'), { target: { value: 'Java 25 SMP' } });
    fireEvent.change(screen.getByLabelText('Java'), { target: { value: '25' } });
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Buat & provision' })); });

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const request = fetchMock.mock.calls[0][1] as RequestInit;
    expect(JSON.parse(String(request.body))).toMatchObject({ name: 'Java 25 SMP', runtime: 'paper', javaVersion: 25 });
  });
});
