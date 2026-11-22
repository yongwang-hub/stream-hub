import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, request, setAuthHandlers } from './api';

afterEach(() => {
  vi.unstubAllGlobals();
  setAuthHandlers({});
});

const jsonResponse = (body: unknown, status = 200): Response =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });

describe('request', () => {
  it('解包 code=0 的 data', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ code: 0, msg: 'ok', data: { id: 7 } })),
    );
    await expect(request<{ id: number }>('/x')).resolves.toEqual({ id: 7 });
  });

  it('业务错误抛出带 code 的 ApiError', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ code: 20003, msg: 'slug 已存在' })),
    );
    await expect(request('/x')).rejects.toMatchObject({
      name: 'ApiError',
      code: 20003,
      message: 'slug 已存在',
    });
  });

  it('Huma 的 problem+json 错误提取 detail 文案', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        new Response(
          JSON.stringify({
            type: 'about:blank',
            title: 'Unprocessable Entity',
            status: 422,
            detail: '产品名不能为空',
          }),
          { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    );
    await expect(request('/x')).rejects.toMatchObject({
      name: 'ApiError',
      code: 422,
      message: '产品名不能为空',
    });
  });

  it('网络异常转成 ApiError(-1)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('offline');
      }),
    );
    await expect(request('/x')).rejects.toBeInstanceOf(ApiError);
  });

  it('HTTP 401 触发全局登出回调', async () => {
    const onUnauthorized = vi.fn();
    setAuthHandlers({ onUnauthorized });
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ code: 90004, msg: 'unauth' }, 401)),
    );
    await expect(request('/x')).rejects.toMatchObject({ code: 90004 });
    expect(onUnauthorized).toHaveBeenCalledOnce();
  });
});
