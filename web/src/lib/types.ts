// 接口类型从 OpenAPI 生成的 schema 派生（pnpm openapi 自动更新），
// 只保留 SSE 等不在 OpenAPI 中的前端专用类型。
import type { components } from './api-schema';

type Schemas = components['schemas'];

export type ChannelStatus = 'idle' | 'live' | 'error' | 'stopped';

// 状态在 schema 里是 string，这里收窄回状态机字面量，保留前端强类型。
export type Channel = Omit<Schemas['Channel'], 'status'> & { status: ChannelStatus };

// 创建频道只需要这三个字段（status 由后端状态机决定）。
export type ChannelInput = Pick<Schemas['ChannelRequest'], 'slug' | 'name' | 'source_url'>;

// 登录与改密返回同一结构（改密后 must_change 恒为 false）。
export type TokenResult = Schemas['TokenResult'];
export type LoginResult = Schemas['TokenResult'];

// ---- SSE 专用类型（不走 OpenAPI 自动生成）----

export type ChannelEventType =
  | 'channel.created'
  | 'channel.updated'
  | 'channel.deleted'
  | 'channel.status'
  | 'probe'
  | 'message';

export interface StreamEvent<T = unknown> {
  id: string;
  type: string;
  data: T;
  receivedAt: number;
}

export type StreamStatus = 'connecting' | 'open' | 'closed';
