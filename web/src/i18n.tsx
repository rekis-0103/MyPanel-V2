import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

export type Locale = 'id' | 'en';
type I18nValue = { locale: Locale; setLocale: (locale: Locale) => void; tr: (id: string, en: string) => string };
const I18nContext = createContext<I18nValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, updateLocale] = useState<Locale>(() => localStorage.getItem('mypanel.locale') === 'en' ? 'en' : 'id');
  useEffect(() => { document.documentElement.lang = locale; }, [locale]);
  const value = useMemo<I18nValue>(() => ({
    locale,
    setLocale(next) { localStorage.setItem('mypanel.locale', next); updateLocale(next); },
    tr: (id, en) => locale === 'id' ? id : en,
  }), [locale]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const value = useContext(I18nContext);
  if (!value) throw new Error('useI18n must be used inside I18nProvider');
  return value;
}
