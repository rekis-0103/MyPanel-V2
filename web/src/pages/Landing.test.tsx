import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it } from 'vitest';
import { I18nProvider } from '../i18n';
import { Landing } from './Landing';

function renderLanding() {
  return render(<MemoryRouter><I18nProvider><Landing /></I18nProvider></MemoryRouter>);
}

describe('Landing', () => {
  afterEach(() => {
    document.documentElement.dataset.theme = 'dark';
    localStorage.clear();
  });

  it('links the primary actions to the login page', () => {
    renderLanding();
    expect(screen.getByRole('link', { name: /Mulai sekarang/i })).toHaveAttribute('href', '/login');
    expect(screen.getByRole('link', { name: /Buka MyPanel/i })).toHaveAttribute('href', '/login');
  });

  it('switches language and theme without losing the page', () => {
    renderLanding();
    fireEvent.click(screen.getByRole('button', { name: 'Gunakan bahasa Inggris' }));
    expect(screen.getByRole('heading', { name: 'Host Minecraft. Skip busywork.' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Use light mode' }));
    expect(document.documentElement.dataset.theme).toBe('light');
    expect(localStorage.getItem('mypanel.theme')).toBe('light');
  });
});
