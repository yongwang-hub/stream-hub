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

// Token 只放内存：刷新页面 / 重开地址都要重新登录，打开站点先看到登录页。
// “记住我”只记用户名（Login.svelte 的 localStorage key），不记凭据。
let memoryToken: string | null = null;

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

// 后端 msg 是英文，这里映射成面向用户的中文（覆盖常见凭据/权限类错误）。
const MESSAGE_I18N: Record<string, string> = {
  'invalid credentials': '用户名或密码错误',
  'account not found': '用户名或密码错误',
  'old password incorrect': '当前密码不正确',
  'account disabled': '账号已被停用，请联系管理员',
};

const translate = (msg: string): string => MESSAGE_I18N[msg] ?? msg;

let handlers: AuthHandlers = {};
export function setAuthHandlers(next: AuthHandlers): void {
  handlers = next;
}

export function getToken(): string | null {
  return memoryToken;
}
export function setToken(token: string): void {
  memoryToken = token;
}
export function clearToken(): void {
  memoryToken = null;
}

export async function request<T>(
  path: string,
  init: RequestInit = {},
  opts: { auth?: boolean } = {},
): Promise<T> {
  // auth=true（默认）：401 视为 token 失效 → 清 token、回调跳登录页。
  // auth=false：登录 / 改密这类“凭据校验”接口，401 表示的是密码不对，
  // 走下面的业务分支按 body.msg 报错，绝不能清 token 或踢回登录页。
  const authSignout = opts.auth !== false;

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
  if (response.status === 401 && authSignout) {
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
      body.msg ? translate(body.msg) : `HTTP ${response.status}`,
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
  // login / changePassword 都可能因“密码不对”回 401，必须 auth:false，
  // 否则会被当成 token 失效清掉登录态（旧密码输错就被踢回登录页的 bug）。
  login: (username: string, password: string) =>
    request<LoginResult>(
      '/login',
      {
        method: 'POST',
        body: json({ username, password }),
      },
      { auth: false },
    ),

  changePassword: (oldPassword: string, newPassword: string) =>
    request<TokenResult>(
      '/me/password',
      {
        method: 'PUT',
        body: json({ old_password: oldPassword, new_password: newPassword }),
      },
      { auth: false },
    ),

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
    request<AdminUser>(`/api/v1/users/${id}`, {
      method: 'PUT',
      body: json(input),
    }),
  deleteAdminUser: (id: number) =>
    request<{ deleted: number }>(`/api/v1/users/${id}`, { method: 'DELETE' }),

  // ---- 角色管理（真接口 /api/v1/roles）----
  listRoles: () => request<AdminRole[]>('/api/v1/roles'),
  createRole: (input: AdminRoleInput) =>
    request<AdminRole>('/api/v1/roles', { method: 'POST', body: json(input) }),
  updateRole: (id: number, input: AdminRoleInput) =>
    request<AdminRole>(`/api/v1/roles/${id}`, {
      method: 'PUT',
      body: json(input),
    }),
  deleteRole: (id: number) =>
    request<{ deleted: number }>(`/api/v1/roles/${id}`, { method: 'DELETE' }),
};
