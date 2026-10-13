import type { Session } from '../types'
import { request } from '../http'
import { getLocale } from '@/i18n'
import { memberResources, type PreloadResource } from './preloadRoutes'

let active: AbortController | null = null
const PRELOAD_CONCURRENCY = 5

export function stopSessionPreload(): void {
  active?.abort()
  active = null
}

export function preloadSession(session: Session): void {
  stopSessionPreload()
  if (session.captcha?.required) return
  const controller = new AbortController()
  active = controller
  const queue = memberResources(session, getLocale())
  const seen = new Set<string>()

  async function load(resource: PreloadResource): Promise<void> {
    const identity = JSON.stringify(resource)
    if (seen.has(identity)) return
    seen.add(identity)
    const timeout = new AbortController()
    const abort = () => timeout.abort()
    controller.signal.addEventListener('abort', abort, { once: true })
    const timer = globalThis.setTimeout(abort, 20_000)
    try {
      await request(resource.path, { ...resource.options, signal: timeout.signal })
    } catch (error) {
      if ((error as { status?: number } | null)?.status === 401) controller.abort()
      // Optional background reads never replace the foreground page's error handling.
    } finally {
      globalThis.clearTimeout(timer)
      controller.signal.removeEventListener('abort', abort)
    }
  }

  async function worker(): Promise<void> {
    while (!controller.signal.aborted && active === controller) {
      const resource = queue.shift()
      if (!resource) return
      await load(resource)
    }
  }

  // Start after bootstrap can mount the requested page. Five reads run at a time.
  globalThis.setTimeout(() => {
    if (active !== controller || controller.signal.aborted) return
    void Promise.allSettled(Array.from({ length: PRELOAD_CONCURRENCY }, worker))
  }, 0)
}
