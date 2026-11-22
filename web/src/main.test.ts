import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import shell from '../index.html?raw'

const { mountMiniApp, browserLoaded, subscriptionLoaded } = vi.hoisted(() => ({
  mountMiniApp: vi.fn((sdkReady: Promise<void>) => sdkReady),
  browserLoaded: vi.fn(),
  subscriptionLoaded: vi.fn(),
}))
vi.mock('./miniapp', () => ({ mountMiniApp }))
vi.mock('./browser/main', () => { browserLoaded(); return {} })
vi.mock('./publicSubscription/main', () => { subscriptionLoaded(); return {} })

const originalTelegram = window.Telegram
const injectedTelegram = { WebApp: { version: '9.0', initData: 'synthetic-test-data' } } as typeof window.Telegram
let sdkScript: HTMLScriptElement | undefined

beforeEach(() => {
  vi.resetModules()
  vi.clearAllMocks()
  sdkScript = undefined
  window.Telegram = undefined
  history.replaceState(null, '', '/#tgWebAppVersion=9.0')
  const parsed = new DOMParser().parseFromString(shell, 'text/html')
  const element = parsed.querySelector('#app')
  if (!element) throw new Error('Missing app container')
  document.body.append(document.importNode(element, true))
  vi.spyOn(document.head, 'append').mockImplementation(node => {
    if (node instanceof HTMLScriptElement) sdkScript = node
  })
})

afterEach(() => {
  vi.restoreAllMocks()
  window.Telegram = originalTelegram
  delete document.documentElement.dataset.experience
  document.body.replaceChildren()
  history.replaceState(null, '', '/')
})

async function loadMemberEntry(): Promise<Promise<void>[]> {
  await import('./main')
  await vi.dynamicImportSettled()
  expect(mountMiniApp).toHaveBeenCalledOnce()
  expect(browserLoaded).not.toHaveBeenCalled()
  expect(subscriptionLoaded).not.toHaveBeenCalled()
  return mountMiniApp.mock.calls[0] ?? []
}

describe('member startup dispatch', () => {
  it('loads the member entry while the SDK remains pending', async () => {
    const [sdkReady] = await loadMemberEntry()
    const settled = vi.fn()
    void sdkReady?.then(settled)
    await Promise.resolve()
    expect(settled).not.toHaveBeenCalled()
    expect(document.querySelector('#app')?.childElementCount).toBe(0)
    expect(shell).not.toContain('id="member-startup"')
    expect(sdkScript?.src).toContain('https://telegram.org/js/telegram-web-app.js')

    window.Telegram = injectedTelegram
    sdkScript?.dispatchEvent(new Event('load'))
    await expect(sdkReady).resolves.toBeUndefined()
  })

  it('reuses an already installed SDK', async () => {
    window.Telegram = injectedTelegram
    const [sdkReady] = await loadMemberEntry()
    expect(sdkScript).toBeUndefined()
    await expect(sdkReady).resolves.toBeUndefined()
  })

  it.each(['error', 'load'] as const)('shows native recovery on SDK %s without a usable WebApp', async event => {
    const [sdkReady] = await loadMemberEntry()
    sdkScript?.dispatchEvent(new Event(event))
    await expect(sdkReady).rejects.toThrow('SDK_UNAVAILABLE')
    await vi.waitFor(() => expect(document.querySelector('#app [role="alert"] button')).not.toBeNull())
    expect(document.querySelector('#app [role="status"]')).toBeNull()
    expect(sdkScript?.onload).toBeNull()
    expect(sdkScript?.onerror).toBeNull()
  })

  it('times out a stalled SDK and clears its event handlers', async () => {
    const timer = vi.spyOn(globalThis, 'setTimeout')
    const [sdkReady] = await loadMemberEntry()
    const expire = timer.mock.calls.find(([, delay]) => delay === 10_000)?.[0]
    expect(typeof expire).toBe('function')
    if (typeof expire === 'function') expire()
    await expect(sdkReady).rejects.toThrow('SDK_TIMEOUT')
    await vi.waitFor(() => expect(document.querySelector('#app [role="alert"] button')).not.toBeNull())
    expect(sdkScript?.onload).toBeNull()
    expect(sdkScript?.onerror).toBeNull()
  })

  it.each([
    ['/admin/login', browserLoaded, subscriptionLoaded],
    ['/subscription/synthetic', subscriptionLoaded, browserLoaded],
  ] as const)('does not request the SDK or member entry on %s', async (path, selected, other) => {
    history.replaceState(null, '', `${path}#tgWebAppVersion=9.0`)
    await import('./main')
    await vi.dynamicImportSettled()
    expect(selected).toHaveBeenCalledOnce()
    expect(other).not.toHaveBeenCalled()
    expect(mountMiniApp).not.toHaveBeenCalled()
    expect(sdkScript).toBeUndefined()
  })
})
