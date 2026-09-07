import { useCallback, useEffect, useState } from 'react';
import { api } from './api';
import type { Metrics, MetricsBatch } from './types';

export function useServerMetrics(serverIds: string[], onError?: (error: unknown) => void) {
  const [metrics, setMetrics] = useState<Record<string, Metrics | null>>({});
  const key = serverIds.join(',');
  const load = useCallback(async () => {
    if (!serverIds.length) { setMetrics({}); return; }
    try {
      const result = await api<MetricsBatch>('/api/v1/metrics/servers');
      setMetrics(Object.fromEntries(serverIds.map((id) => [id, result.items[id] ?? null])));
    } catch (error) { onError?.(error); }
  // key is the stable identity of the requested server set.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, onError]);
  useEffect(() => {
    let active = true;
    const refresh = async () => { if (active) await load(); };
    refresh();
    const timer = window.setInterval(refresh, 5000);
    return () => { active = false; window.clearInterval(timer); };
  }, [load]);
  return metrics;
}
