import { computed, shallowRef } from 'vue'

export type BrowserTheme = 'system' | 'light' | 'dark'
const storageKey = 'txc-browser-theme'
const preference = shallowRef<BrowserTheme>('system')
const systemDark = shallowRef(false)
const dark = computed(() => preference.value === 'dark' || (preference.value === 'system' && systemDark.value))

function applyTheme(): void {
  document.documentElement.classList.toggle('dark', dark.value)
  document.documentElement.style.colorScheme = dark.value ? 'dark' : 'light'
}

export function setBrowserTheme(next: BrowserTheme): void {
  preference.value = next
  try { localStorage.setItem(storageKey, next) } catch { /* Session preference remains usable. */ }
  applyTheme()
}

export function initializeBrowserTheme(): () => void {
  try {
    const saved = localStorage.getItem(storageKey)
    if (saved === 'dark' || saved === 'light') preference.value = saved
  } catch { /* System preference is the default. */ }
  const query = matchMedia('(prefers-color-scheme: dark)')
  const update = () => { systemDark.value = query.matches; applyTheme() }
  query.addEventListener('change', update)
  update()
  return () => query.removeEventListener('change', update)
}

export function useBrowserTheme() { return { preference, dark, setTheme: setBrowserTheme } }
