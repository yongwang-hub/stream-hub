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
  // 与 Vue 源码一致同时维护 light/dark 两个 class，供 View Transition 伪元素选择层级。
  root.classList.toggle('dark', theme === 'dark');
  root.classList.toggle('light', theme === 'light');
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

// 圆形扩散切换：照公司 Vue xc-system/setSystemTheme 用 View Transitions API。
// 从点击处扩散，400ms ease-in；浏览器不支持 startViewTransition 时直接切换、无动画。
export function toggleThemeWithCircularReveal(event: MouseEvent): Theme {
  const next: Theme = currentTheme() === 'dark' ? 'light' : 'dark';
  const toDark = next === 'dark';

  if (!document.startViewTransition) {
    applyTheme(next);
    return next;
  }

  const transition = document.startViewTransition(() => {
    applyTheme(next);
  });

  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  const x = rect.left + rect.width / 2;
  const y = rect.top + rect.height / 2;
  const endRadius = Math.hypot(
    Math.max(x, window.innerWidth - x),
    Math.max(y, window.innerHeight - y),
  );
  const ratioX = (100 * x) / window.innerWidth;
  const ratioY = (100 * y) / window.innerHeight;
  const ratioR =
    (100 * endRadius) / (Math.hypot(window.innerWidth, window.innerHeight) / Math.SQRT2);

  transition.ready.then(() => {
    const clipPath = [
      `circle(0% at ${ratioX}% ${ratioY}%)`,
      `circle(${ratioR}% at ${ratioX}% ${ratioY}%)`,
    ];
    // 切到暗色：旧快照(浅色)从大圆收缩到 0，露出新主题；切到亮色：新快照从小圆扩张。
    document.documentElement.animate(
      { clipPath: toDark ? [...clipPath].reverse() : clipPath },
      {
        duration: 400,
        easing: 'ease-in',
        pseudoElement: toDark ? '::view-transition-old(root)' : '::view-transition-new(root)',
      },
    );
  });

  return next;
}
