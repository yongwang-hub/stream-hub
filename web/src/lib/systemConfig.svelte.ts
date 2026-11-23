// 系统配置的响应式状态：登录后从后端 GET 加载，保存走 PUT；不再存 localStorage。
// 加载前的初始值仅作占位（保证登录页标题不空），不是缓存——数据以服务端为准。
import { api, type SystemConfig, type SystemConfigWritable } from './api';

const PLACEHOLDER: SystemConfig = {
  product_name: '流媒体输出分发平台',
  product_model: 'STREAM-HUB-1000',
  serial_number: 'SH-2026-0000-0001',
  product_version: 'V1.0.0',
  login_bg: '',
  product_icon: '',
  warning_text:
    '本系统视频图像严禁拍摄、截屏、录像，禁止任何形式的信息外传，违者依法追责！',
  watermark: false,
  license_days: 1090,
  machine_code: '',
  activation_code: '',
  started_at: '',
};

// 模块级 $state：通过 config() 函数读取以保留组件内响应性。
let state = $state<SystemConfig>({ ...PLACEHOLDER });

export function config(): SystemConfig {
  return state;
}

/** 从后端拉取配置覆盖本地状态；登录页阶段无 token 时调用会失败，保留占位值即可。 */
export async function loadConfig(): Promise<void> {
  try {
    state = await api.getSystemConfig();
  } catch {
    // 未登录 / 后端不可用：保留占位值，不阻塞页面渲染。
  }
}

/** 保存可改字段，成功后用后端返回值更新状态（只读字段以服务端为准）。 */
export async function saveConfig(input: SystemConfigWritable): Promise<void> {
  const next = await api.updateSystemConfig(input);
  state = next;
}

export { PLACEHOLDER as DEFAULT_CONFIG };
