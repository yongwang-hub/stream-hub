// 主题：只保留 light / dark 两套（用户确认），切换根元素的 .dark class。
// 主题色由 app.css 的 CSS 变量统一提供，组件不写死颜色。
export type Theme = 'light' | 'dark';

const KEY = 'stream-hub.theme';

const storage = (): Storage | null =>
  typeof localStorage === 'undefined' ? null : localStorage;

export function currentTheme(): Theme {
  const saved = storage()?.getItem(KEY);
  if (saved === 'light' || saved === 'dark') return saved;
  if (
    typeof window !== 'undefined' &&
    window.matchMedia('(prefers-color-scheme: dark)').matches
  ) {
    return 'dark';
  }
  return 'light';
}

export function applyTheme(theme: Theme): void {
  const root = document.documentElement;
  root.classList.toggle('dark', theme === 'dark');
  root.style.colorScheme = theme;
  storage()?.setItem(KEY, theme);
}

export function initTheme(): void {
  applyTheme(currentTheme());
}

export function toggleTheme(): Theme {
  const next: Theme = currentTheme() === 'dark' ? 'light' : 'dark';
  applyTheme(next);
  return next;
}
