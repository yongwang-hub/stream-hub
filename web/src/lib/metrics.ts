// 系统指标客户端：首屏用快照接口补齐历史，实时增量走 SSE 频道 system-metrics。
// 与 sse.ts 分开是因为指标事件固定是 "point"，且需要独立的连接状态给监控页降级显示。
import { getToken } from './api';

export interface MetricPoint {
  time: string; // RFC3339
  cpu: number;
  mem_used: number;
  mem_total: number;
  mem_percent: number;
  net_rx: number; // Mbps
  net_tx: number; // Mbps
  disk_used: number;
  disk_total: number;
  disk_percent: number;
  load1: number; // Windows 不支持时为 -1
  load5: number;
  load15: number;
  goroutines: number;
  heap_alloc: number;
}

export interface MetricsSnapshot {
  latest: MetricPoint | null;
  history: MetricPoint[];
}

export type MetricsStatus = 'connecting' | 'open' | 'closed';

const authHeader = (): HeadersInit => {
  const token = getToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
};

/** 首屏快照：返回固定容量内的历史点（最新在最后）。 */
export async function fetchMetricsSnapshot(): Promise<MetricsSnapshot> {
  const res = await fetch('/api/v1/metrics', { headers: authHeader() });
  const body = (await res.json()) as {
    code: number;
    msg: string;
    data?: MetricsSnapshot;
  };
  if (!res.ok || body.code !== 0 || !body.data) {
    throw new Error(body.msg || `HTTP ${res.status}`);
  }
  return body.data;
}

/** 订阅实时增量；EventSource 自带断线重连，close() 后彻底断开。 */
export function subscribeMetrics(
  onPoint: (point: MetricPoint) => void,
  onStatus: (status: MetricsStatus) => void,
): { close: () => void } {
  const params = new URLSearchParams();
  const token = getToken();
  if (token) params.set('token', token); // EventSource 不能自定义 Authorization 头
  const source = new EventSource(
    `/api/v1/stream/system-metrics?${params.toString()}`,
  );
  onStatus('connecting');
  source.onopen = () => onStatus('open');
  source.onerror = () => onStatus('closed'); // 浏览器会自动重连，onopen 时状态恢复
  source.addEventListener('point', (event) => {
    try {
      onPoint(JSON.parse((event as MessageEvent<string>).data) as MetricPoint);
    } catch {
      // 坏包直接丢弃，不打断监控页
    }
  });
  return {
    close() {
      source.close();
      onStatus('closed');
    },
  };
}
