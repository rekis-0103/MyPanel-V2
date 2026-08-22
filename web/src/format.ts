export function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`;
}

export function formatDate(value: string) {
  const locale = localStorage.getItem('mypanel.locale') === 'en' ? 'en-US' : 'id-ID';
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}

export function serverAddress(bindIp: string, port: number, panelHostname = window.location.hostname) {
  const host = bindIp === '0.0.0.0' || bindIp === '::' || bindIp === '' ? panelHostname : bindIp;
  return `${host}:${port}`;
}
