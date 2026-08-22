import React from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { App } from './App';
import { ToastProvider } from './components/ui/Toast';
import { I18nProvider } from './i18n';
import './styles/tokens.css';
import './style.css';

document.documentElement.dataset.theme = localStorage.getItem('mypanel.theme') === 'light' ? 'light' : 'dark';

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <I18nProvider><ToastProvider><BrowserRouter><App /></BrowserRouter></ToastProvider></I18nProvider>
  </React.StrictMode>,
);
