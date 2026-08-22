export function MetricBar({ label, value, max, unit = '', warningThreshold = 70, dangerThreshold = 90, detail }: {
  label: string; value: number; max: number; unit?: string; warningThreshold?: number; dangerThreshold?: number; detail?: string;
}) {
  const percent = max > 0 ? Math.min(100, Math.max(0, value / max * 100)) : 0;
  const level = percent >= dangerThreshold ? 'danger' : percent >= warningThreshold ? 'warning' : 'healthy';
  return <div className="metric-bar">
    <div className="metric-label"><span>{label}</span><b>{value.toLocaleString()} {unit}</b></div>
    <progress className={`metric-track ${level}`} aria-label={label} max={max || 100} value={max > 0 ? value : 0} />
    <small>{detail ?? `${percent.toFixed(0)}%`}</small>
  </div>;
}
