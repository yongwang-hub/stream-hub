// 全局消息提示（参考 Element Plus ElMessage）：任意模块 import { toast } 直接弹。
// 文件名必须是 .svelte.ts，模块级 $state 才能让 Toaster 组件响应式渲染。
export type ToastType = 'success' | 'error' | 'warning' | 'info';

export interface ToastItem {
  id: number;
  type: ToastType;
  message: string;
}

let items = $state<ToastItem[]>([]);
let seq = 0;

// 停留时长：错误多给一点时间看清楚，成功短一点不碍事。
const DURATION: Record<ToastType, number> = {
  success: 2600,
  info: 3200,
  warning: 3600,
  error: 4000,
};

function push(type: ToastType, message: string): void {
  const id = ++seq;
  items.push({ id, type, message });
  setTimeout(() => dismiss(id), DURATION[type]);
}

export function dismiss(id: number): void {
  items = items.filter((t) => t.id !== id);
}

// Svelte 5 模块状态需通过函数读取才能保持组件响应性（直接导出变量会丢失引用）。
export function toastItems(): ToastItem[] {
  return items;
}

export const toast = {
  success: (message: string): void => push('success', message),
  error: (message: string): void => push('error', message),
  warning: (message: string): void => push('warning', message),
  info: (message: string): void => push('info', message),
};
