// 统一封装后端 {code,msg,data} 信封、Bearer token 与两类全局鉴权跳转。
import type { components } from './api-schema';
import type { Channel, ChannelInput, LoginResult, TokenResult } from './types';

export interface Envelope<T = unknown> {
  code: number;
  msg: string;
  data?: T;
}

// Huma 接口的错误用 RFC9457 problem+json（application/problem+json），
// 字段是 type/title/status/detail，与项目信封不同。统一在这里识别。
interface ProblemDetail {
  type?: string;
  title?: string;
  status?: number;
  detail?: string;
}

function isProblem(body: unknown): body is ProblemDetail {
  return (
    typeof body === 'object' &&
    body !== null &&
    'detail' in body &&
    typeof (body as ProblemDetail).detail === 'string'
  );
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

  let body: Envelope<T> | ProblemDetail;
  try {
    body = (await response.json()) as Envelope<T> | ProblemDetail;
  } catch {
    throw new ApiError(
      response.status,
      `响应不是 JSON（HTTP ${response.status}）`,
    );
  }
  if (!response.ok) {
    if (isProblem(body)) {
      throw new ApiError(
        body.status || response.status,
        translate(body.detail || body.title || `HTTP ${response.status}`),
      );
    }
    const env = body as Envelope<T>;
    throw new ApiError(
      env.code || response.status,
      env.msg ? translate(env.msg) : `HTTP ${response.status}`,
    );
  }
  const env = body as Envelope<T>;
  if (env.code !== CODE_OK) {
    // 10100 不是错误：首次登录成功但必须先改密，响应里的 token 仍需交给调用方
    // （丢了它，强制改密弹窗的请求就会因缺 Bearer token 而失败）。
    if (env.code === CODE_PASSWORD_CHANGE_REQUIRED) {
      handlers.onPasswordChangeRequired?.();
      return env.data as T;
    }
    throw new ApiError(env.code, env.msg || `业务错误 ${env.code}`);
  }
  return env.data as T;
}

const json = (body: unknown): string => JSON.stringify(body);

// 以下类型均由 OpenAPI schema 自动派生（pnpm openapi），不再手写。
type Schemas = components['schemas'];

/** 用户管理：GET/POST/PUT/DELETE /api/v1/users。 */
export type AdminUser = Schemas['AdminUser'];
export type AdminUserInput = Schemas['AdminUserInput'];
export type AdminUserPage = Schemas['PageAdminUser'];

/** 角色管理：GET/POST/PUT/DELETE /api/v1/roles。 */
export type AdminRole = Schemas['AdminRole'];
export type AdminRoleInput = Schemas['AdminRoleInput'];
export type AdminRolePage = Schemas['PageAdminRole'];

/** 频道列表分页载荷。 */
export type ChannelPage = Schemas['PageChannel'];

/** 摄像机（设备管理）：列表/详情不含密码，输入可带密码。 */
export type Camera = Schemas['Camera'];
export type CameraInput = Schemas['CameraInput'];
export type CameraPage = Schemas['PageCamera'];

/** 实时预览：左侧设备列表（不分页，含停用设备）。 */
export type PreviewChannel = Schemas['PreviewChannel'];
export type PreviewChannelList = Schemas['PreviewChannelList'];

/** 系统配置：可改字段落库、只读元数据由后端启动时采集一次。 */
export type SystemConfig = Schemas['Config'];
/** 系统配置里允许修改的字段子集。 */
export type SystemConfigWritable = Schemas['Writable'];

export const api = {
  // login / changePassword 都可能因“密码不对”回 401，必须 auth:false，
  // 否则会被当成 token 失效清掉登录态（旧密码输错就被踢回登录页的 bug）。
  login: (username: string, password: string) =>
    request<LoginResult>(
      '/api/v1/login',
      {
        method: 'POST',
        body: json({ username, password }),
      },
      { auth: false },
    ),

  changePassword: (oldPassword: string, newPassword: string) =>
    request<TokenResult>(
      '/api/v1/me/password',
      {
        method: 'PUT',
        body: json({ old_password: oldPassword, new_password: newPassword }),
      },
      { auth: false },
    ),

  listChannels: (page = 1, pageSize = 20) =>
    request<ChannelPage>(`/api/v1/channels?page=${page}&page_size=${pageSize}`),

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

  // ---- 设备管理 / 摄像机（真接口 /api/v1/cameras）----
  listCameras: (
    params: {
      page?: number;
      pageSize?: number;
      keyword?: string;
      enabled?: boolean;
    } = {},
  ) => {
    const q = new URLSearchParams({
      page: String(params.page ?? 1),
      page_size: String(params.pageSize ?? 20),
    });
    if (params.keyword) q.set('keyword', params.keyword);
    if (params.enabled !== undefined) q.set('enabled', String(params.enabled));
    return request<CameraPage>(`/api/v1/cameras?${q}`);
  },
  getCamera: (id: number) => request<Camera>(`/api/v1/cameras/${id}`),
  createCamera: (input: CameraInput) =>
    request<Camera>('/api/v1/cameras', { method: 'POST', body: json(input) }),
  updateCamera: (id: number, input: CameraInput) =>
    request<Camera>(`/api/v1/cameras/${id}`, {
      method: 'PUT',
      body: json(input),
    }),
  deleteCamera: (id: number) =>
    request<{ deleted: number }>(`/api/v1/cameras/${id}`, { method: 'DELETE' }),

  // ---- 实时预览（/api/v1/preview/cameras，不分页）----
  previewCameras: (keyword = '') => {
    const q = keyword ? `?keyword=${encodeURIComponent(keyword)}` : '';
    return request<PreviewChannelList>(`/api/v1/preview/cameras${q}`);
  },

  publishEvent: (channel: string, type: string, data: unknown) =>
    request<{ id: string; channel: string }>(
      `/api/v1/stream/${encodeURIComponent(channel)}/events`,
      { method: 'POST', body: json({ type, data }) },
    ),

  // ---- 用户管理（真接口 /api/v1/users）----
  listAdminUsers: (page = 1, pageSize = 20) =>
    request<AdminUserPage>(`/api/v1/users?page=${page}&page_size=${pageSize}`),
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
  listRoles: (page = 1, pageSize = 20) =>
    request<AdminRolePage>(`/api/v1/roles?page=${page}&page_size=${pageSize}`),
  createRole: (input: AdminRoleInput) =>
    request<AdminRole>('/api/v1/roles', { method: 'POST', body: json(input) }),
  updateRole: (id: number, input: AdminRoleInput) =>
    request<AdminRole>(`/api/v1/roles/${id}`, {
      method: 'PUT',
      body: json(input),
    }),
  deleteRole: (id: number) =>
    request<{ deleted: number }>(`/api/v1/roles/${id}`, { method: 'DELETE' }),

  // ---- 系统配置（真接口 /api/v1/system-config）----
  getSystemConfig: () => request<SystemConfig>('/api/v1/system-config'),
  updateSystemConfig: (input: SystemConfigWritable) =>
    request<SystemConfig>('/api/v1/system-config', {
      method: 'PUT',
      body: json(input),
    }),
};

/** 上传图片（系统配置的背景/图标）：FormData 不能带 application/json 头，故走独立 fetch。 */
export async function uploadImage(file: File): Promise<string> {
  const form = new FormData();
  form.append('file', file);
  const headers = new Headers();
  const token = getToken();
  if (token) headers.set('Authorization', `Bearer ${token}`);

  let response: Response;
  try {
    response = await fetch('/api/v1/system-config/upload', {
      method: 'POST',
      headers,
      body: form,
    });
  } catch (err) {
    throw new ApiError(
      -1,
      `网络错误：${err instanceof Error ? err.message : String(err)}`,
    );
  }
  if (response.status === 401) {
    clearToken();
    handlers.onUnauthorized?.();
    throw new ApiError(CODE_UNAUTHENTICATED, '登录已失效，请重新登录');
  }
  let body: Envelope<{ url: string }> | ProblemDetail;
  try {
    body = (await response.json()) as Envelope<{ url: string }> | ProblemDetail;
  } catch {
    throw new ApiError(
      response.status,
      `响应不是 JSON（HTTP ${response.status}）`,
    );
  }
  if (!response.ok) {
    if (isProblem(body)) {
      throw new ApiError(
        body.status || response.status,
        translate(
          body.detail || body.title || `上传失败（HTTP ${response.status}）`,
        ),
      );
    }
    const env = body as Envelope<{ url: string }>;
    throw new ApiError(
      env.code || response.status,
      env.msg || `上传失败（HTTP ${response.status}）`,
    );
  }
  const env = body as Envelope<{ url: string }>;
  if (env.code !== CODE_OK) {
    throw new ApiError(
      env.code,
      env.msg || `上传失败（HTTP ${response.status}）`,
    );
  }
  if (!env.data) {
    throw new ApiError(env.code, '上传成功但响应缺少文件地址');
  }
  return env.data.url;
}
