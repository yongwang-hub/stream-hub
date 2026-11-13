// 统一封装后端 {code,msg,data} 信封、Bearer token 与两类全局鉴权跳转。
import type { Channel, ChannelInput, LoginResult, TokenResult } from './types';

export interface Envelope<T = unknown> {
  code: number;
  msg: string;
  data?: T;
}

export const CODE_OK = 0;
export const CODE_PASSWORD_CHANGE_REQUIRED = 10100;
export const CODE_UNAUTHENTICATED = 90004;

const TOKEN_KEY = 'stream-hub.token';

export class ApiError extends Error {
  readonly code: number;

  constructor(code: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
  }
}

export interface AuthHandlers {
  onUnauthorized?: () => void;
  onPasswordChangeRequired?: () => void;
}

let handlers: AuthHandlers = {};
export function setAuthHandlers(next: AuthHandlers): void {
  handlers = next;
}

// Node 测试环境没有 localStorage，所有存取都做空值保护。
const storage = (): Storage | null =>
  typeof localStorage === 'undefined' ? null : localStorage;
export function getToken(): string | null {
  return storage()?.getItem(TOKEN_KEY) ?? null;
}
export function setToken(token: string): void {
  storage()?.setItem(TOKEN_KEY, token);
}
export function clearToken(): void {
  storage()?.removeItem(TOKEN_KEY);
}

export async function request<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  const token = getToken();
  if (token) headers.set('Authorization', `Bearer ${token}`);

  let response: Response;
  try {
    response = await fetch(path, { ...init, headers });
  } catch (err) {
    throw new ApiError(
      -1,
      `网络错误：${err instanceof Error ? err.message : String(err)}`,
    );
  }

  // 基础设施 401：token 失效/缺失，直接回调让 App 跳登录。
  if (response.status === 401) {
    clearToken();
    handlers.onUnauthorized?.();
    throw new ApiError(CODE_UNAUTHENTICATED, '登录已失效，请重新登录');
  }

  let body: Envelope<T>;
  try {
    body = (await response.json()) as Envelope<T>;
  } catch {
    throw new ApiError(
      response.status,
      `响应不是 JSON（HTTP ${response.status}）`,
    );
  }
  if (!response.ok) {
    throw new ApiError(
      body.code || response.status,
      body.msg || `HTTP ${response.status}`,
    );
  }
  if (body.code !== CODE_OK) {
    if (body.code === CODE_PASSWORD_CHANGE_REQUIRED)
      handlers.onPasswordChangeRequired?.();
    throw new ApiError(body.code, body.msg || `业务错误 ${body.code}`);
  }
  return body.data as T;
}

const json = (body: unknown): string => JSON.stringify(body);

/** 用户管理：真实账号体系 GET/POST/PUT/DELETE /api/v1/users（sys_users 表）。 */
export interface AdminUser {
  id: number;
  username: string;
  email: string;
  enabled: boolean;
  password_changed: boolean;
  roles: string[];
  role_ids: number[];
}
export interface AdminUserInput {
  username: string;
  password?: string; // 创建必填；更新留空 = 不改密码
  email?: string;
  enabled?: boolean;
  role_ids?: number[];
}

/** 角色管理：真实 RBAC 体系 GET/POST/PUT/DELETE /api/v1/roles（sys_roles 表）。 */
export interface AdminRole {
  id: number;
  name: string;
  description: string;
  permissions: string[];
}
export interface AdminRoleInput {
  name: string;
  description?: string;
  permissions?: string[]; // 不传 = 不改权限；空数组 = 清空
}

/** 系统配置页的接口契约仍待后端实现：GET/PUT /api/v1/config。当前仅本地预览。 */
export interface SystemConfig {
  system_name: string;
  addr: string;
  log_level: string;
  token_ttl: string;
  db_driver: string;
  db_dsn: string;
}

export const api = {
  login: (username: string, password: string) =>
    request<LoginResult>('/login', {
      method: 'POST',
      body: json({ username, password }),
    }),

  changePassword: (oldPassword: string, newPassword: string) =>
    request<TokenResult>('/me/password', {
      method: 'PUT',
      body: json({ old_password: oldPassword, new_password: newPassword }),
    }),

  listChannels: () => request<Channel[]>('/api/v1/channels'),

  createChannel: (input: ChannelInput) =>
    request<Channel>('/api/v1/channels', { method: 'POST', body: json(input) }),

  updateChannel: (
    id: number,
    patch: Partial<Pick<Channel, 'name' | 'source_url' | 'status'>>,
  ) =>
    request<Channel>(`/api/v1/channels/${id}`, {
      method: 'PUT',
      body: json(patch),
    }),

  deleteChannel: (id: number) =>
    request<{ deleted: number }>(`/api/v1/channels/${id}`, {
      method: 'DELETE',
    }),

  publishEvent: (channel: string, type: string, data: unknown) =>
    request<{ id: string; channel: string }>(
      `/api/v1/stream/${encodeURIComponent(channel)}/events`,
      { method: 'POST', body: json({ type, data }) },
    ),

  // ---- 用户管理（真接口 /api/v1/users）----
  listAdminUsers: () => request<AdminUser[]>('/api/v1/users'),
  createAdminUser: (input: AdminUserInput) =>
    request<AdminUser>('/api/v1/users', { method: 'POST', body: json(input) }),
  updateAdminUser: (id: number, input: AdminUserInput) =>
    request<AdminUser>(`/api/v1/users/${id}`, { method: 'PUT', body: json(input) }),
  deleteAdminUser: (id: number) =>
    request<{ deleted: number }>(`/api/v1/users/${id}`, { method: 'DELETE' }),

  // ---- 角色管理（真接口 /api/v1/roles）----
  listRoles: () => request<AdminRole[]>('/api/v1/roles'),
  createRole: (input: AdminRoleInput) =>
    request<AdminRole>('/api/v1/roles', { method: 'POST', body: json(input) }),
  updateRole: (id: number, input: AdminRoleInput) =>
    request<AdminRole>(`/api/v1/roles/${id}`, { method: 'PUT', body: json(input) }),
  deleteRole: (id: number) =>
    request<{ deleted: number }>(`/api/v1/roles/${id}`, { method: 'DELETE' }),
};
