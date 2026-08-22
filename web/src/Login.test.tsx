import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Login } from './App';
import { I18nProvider } from './i18n';

describe('Login', () => {
  beforeEach(() => vi.restoreAllMocks());

  it('submits credentials and returns the owner session', async () => {
    const onLogin = vi.fn();
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ username: 'owner', role: 'owner', csrfToken: 'csrf' }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    vi.stubGlobal('fetch', fetchMock);
    render(<I18nProvider><Login onLogin={onLogin} /></I18nProvider>);
    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'owner' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'a-strong-password' } });
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Masuk' })); });
    await waitFor(() => expect(onLogin).toHaveBeenCalledWith({ username: 'owner', role: 'owner', csrfToken: 'csrf' }));
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/auth/login', expect.objectContaining({ method: 'POST' }));
  });

  it('shows a recoverable login error', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: 'invalid credentials', code: 'invalid_credentials' }), { status: 401, headers: { 'Content-Type': 'application/json' } })));
    render(<I18nProvider><Login onLogin={() => undefined} /></I18nProvider>);
    fireEvent.change(screen.getByLabelText('Username'), { target: { value: 'owner' } });
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'wrong-password' } });
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Masuk' })); });
    expect(await screen.findByRole('alert')).toHaveTextContent('invalid credentials');
  });
});
