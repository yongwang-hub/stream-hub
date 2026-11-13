// SSE 客户端：原生 EventSource 自带断线重连与 Last-Event-ID 头，
// 这里只补三件事：query token、按事件类型分发、手动重建连接时的续传。
import { getToken } from './api';
import type { StreamEvent, StreamStatus } from './types';

// EventSource 只能按具体 event 名注册监听；后端新增事件类型时把名字加进这里即可。
export const KNOWN_EVENTS: readonly string[] = [
  'channel.created',
  'channel.updated',
  'channel.deleted',
  'channel.status',
  'probe',
  'message',
];

export interface StreamOptions {
  channel: string;
  lastEventId?: string;
  onEvent: (event: StreamEvent) => void;
  onStatus?: (status: StreamStatus) => void;
  onEventId?: (id: string) => void;
}

export interface StreamHandle {
  close: () => void;
}

export function openStream(opts: StreamOptions): StreamHandle {
  const params = new URLSearchParams();
  const token = getToken();
  if (token) params.set('token', token); // EventSource 不能带 Authorization 头
  if (opts.lastEventId) params.set('last_event_id', opts.lastEventId);
  const url = `/api/v1/stream/${encodeURIComponent(opts.channel)}?${params.toString()}`;

  const source = new EventSource(url);
  opts.onStatus?.('connecting');
  source.onopen = () => opts.onStatus?.('open');
  source.onerror = () => opts.onStatus?.('connecting'); // 浏览器原生自动重连，不用手工重建

  const listen = (event: Event): void => {
    const message = event as MessageEvent<string>;
    if (message.lastEventId) opts.onEventId?.(message.lastEventId);
    let data: unknown = message.data;
    try {
      data = JSON.parse(message.data) as unknown;
    } catch {
      // 非 JSON 负载保留原始文本
    }
    opts.onEvent({
      id: message.lastEventId,
      type: event.type,
      data,
      receivedAt: Date.now(),
    });
  };
  for (const name of KNOWN_EVENTS) source.addEventListener(name, listen);

  return {
    close() {
      source.close();
      opts.onStatus?.('closed');
    },
  };
}
