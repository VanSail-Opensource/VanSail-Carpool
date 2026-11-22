import { createMemoryHistory, createWebHistory, type RouterHistory } from 'vue-router'

import { isTelegramWebAppDetected } from '@/utils/telegram'
import { pendingRouteRecoveryPath } from './recovery'

function browserRoute(): string {
  const url = new URL(window.location.href)
  const search = new URLSearchParams(url.search)
  for (const key of [...search.keys()]) {
    if (/^tgWebApp(?:Data|Version|Platform|ThemeParams|StartParam)$/i.test(key)) search.delete(key)
  }
  const query = search.toString()
  const hash = /^#tgWebApp(?:Data|Version|Platform|ThemeParams|StartParam)=/i.test(url.hash) ? '' : url.hash
  return `${url.pathname}${query ? `?${query}` : ''}${hash}`
}

export function createAppHistory(telegram = isTelegramWebAppDetected()): RouterHistory {
  if (!telegram) return createWebHistory()

  const history = createMemoryHistory()
  const initialRoute = pendingRouteRecoveryPath() ?? browserRoute()
  if (initialRoute !== '/') history.replace(initialRoute)
  return history
}
