type MetricChartProps = {
  label: string;
  value: number;
  max: number;
  history: number[];
  detail: string;
  formatValue?: (value: number) => string;
};

export function MetricChart({ label, value, max, history, detail, formatValue = (item) => `${item.toFixed(1)}%` }: MetricChartProps) {
  const safeMax = max > 0 ? max : 1;
  const percent = Math.min(100, Math.max(0, value / safeMax * 100));
  const tone = percent >= 90 ? 'danger' : percent >= 70 ? 'warning' : 'healthy';
  const samples = history.length > 1 ? history : [0, ...history];
  const points = samples.map((sample, index) => {
    const x = samples.length === 1 ? 100 : index / (samples.length - 1) * 100;
    const y = 30 - Math.min(1, Math.max(0, sample / safeMax)) * 27;
    return `${x.toFixed(2)},${y.toFixed(2)}`;
  }).join(' ');
  return <article className={`metric-chart ${tone}`}>
    <header><span>{label}</span><b>{formatValue(value)}</b></header>
    <svg viewBox="0 0 100 32" preserveAspectRatio="none" role="img" aria-label={`${label}: ${detail}`}>
      <line x1="0" y1="30" x2="100" y2="30" />
      <polyline points={points} />
    </svg>
    <small>{detail}</small>
  </article>;
}
