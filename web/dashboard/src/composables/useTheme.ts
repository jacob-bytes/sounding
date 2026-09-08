import { ref, watchEffect } from 'vue'

type Theme = 'light' | 'dark'

const KEY = 'sounding-theme'

function initial(): Theme {
  const saved = localStorage.getItem(KEY)
  if (saved === 'light' || saved === 'dark')
    return saved
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

const theme = ref<Theme>(initial())

watchEffect(() => {
  document.documentElement.classList.toggle('dark', theme.value === 'dark')
  localStorage.setItem(KEY, theme.value)
})

/** 亮/暗主题（跟随系统初始化，可手动切换，localStorage 记忆） */
export function useTheme() {
  function toggle() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
  }
  return { theme, toggle }
}
