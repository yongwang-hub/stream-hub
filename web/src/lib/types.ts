// 与后端 internal/media + internal/auth 的 JSON 字段一一对应。

export type ChannelStatus = 'idle' | 'live' | 'error' | 'stopped';

export interface Channel {
  id: number;
  slug: string;
  name: string;
  source_url: string;
  status: ChannelStatus;
  created_at: string;
  updated_at: string;
}

export interface TokenResult {
  token: string;
  expires_in?: number;
}

export interface LoginResult extends TokenResult {
  must_change: boolean;
}

export interface ChannelInput {
  slug: string;
  name: string;
  source_url: string;
}

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
