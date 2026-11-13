import { mount } from 'svelte';
import App from './App.svelte';
import './app.css';
import { initTheme } from './lib/theme';

// 在挂载前应用主题，避免首屏闪一下白/黑。
initTheme();

const target = document.getElementById('app');
if (!target) throw new Error('#app not found');

mount(App, { target });
