import { vi } from 'vitest'
import { config } from '@vue/test-utils'

import { t } from '@/i18n'

config.global.mocks = { $t: t }

const noNetworkFetch = vi.fn(async () => new Response('{}', {
  headers: { 'Content-Type': 'application/json' },
}))
Object.defineProperty(globalThis, 'fetch', { configurable: true, value: noNetworkFetch })
Object.defineProperty(window, 'fetch', { configurable: true, value: noNetworkFetch })

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: query.includes('prefers-reduced-motion: reduce'),
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})

Object.defineProperty(navigator, 'clipboard', {
  configurable: true,
  value: { writeText: vi.fn().mockResolvedValue(undefined) },
})

window.Telegram = {
  WebApp: {
    version: '9.0',
    initData: 'query_id=test',
    initDataUnsafe: {},
    colorScheme: 'dark',
    ready: vi.fn(),
    expand: vi.fn(),
    close: vi.fn(),
    openLink: vi.fn(),
    openTelegramLink: vi.fn(),
    openInvoice: vi.fn(),
    HapticFeedback: {
      impactOccurred: vi.fn(),
      notificationOccurred: vi.fn(),
    },
  },
}
