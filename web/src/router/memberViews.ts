// One loader identity serves navigation and optional idle preloading.
export const memberViews = {
  '/home': () => import('@/views/HomeView.vue'),
  '/subscription': () => import('@/views/SubscriptionView.vue'),
  '/settings': () => import('@/views/SettingsView.vue'),
  '/settings/diy': () => import('@/views/DIYView.vue'),
  '/connections': () => import('@/views/ConnectionsView.vue'),
  '/catalog': () => import('@/views/CatalogView.vue'),
  '/activity': () => import('@/views/ActivityView.vue'),
  '/affiliates': () => import('@/views/AffiliatesView.vue'),
  '/community': () => import('@/views/CommunityView.vue'),
  '/knowledgebase': () => import('@/views/KnowledgebaseView.vue'),
  '/emby': () => import('@/views/EmbyView.vue'),
} as const

type MemberPath = keyof typeof memberViews
const loaded = new Set<MemberPath>()
const pending = new Map<MemberPath, Promise<void>>()
let generation = 0
let idle: number | undefined
let timer: ReturnType<typeof setTimeout> | undefined

function saveData(): boolean {
  const connection = (navigator as Navigator & { connection?: { saveData?: boolean; effectiveType?: string } }).connection
  return connection?.saveData === true || ['slow-2g', '2g'].includes(connection?.effectiveType ?? '')
}

export function preloadMemberView(path: string): Promise<void> {
  if (!(path in memberViews) || saveData()) return Promise.resolve()
  const key = path as MemberPath
  if (loaded.has(key)) return Promise.resolve()
  const existing = pending.get(key)
  if (existing) return existing
  const work = memberViews[key]().then(async () => {
    loaded.add(key)
    if (key === '/catalog') {
      const { preloadCatalogPages } = await import('@/components/catalog/catalogPages')
      await preloadCatalogPages()
    }
  }).catch(() => {
    // Foreground navigation retains its existing chunk recovery and retry UI.
  }).finally(() => { pending.delete(key) })
  pending.set(key, work)
  return work
}

export function stopMemberViewPreload(): void {
  generation++
  if (idle !== undefined) globalThis.cancelIdleCallback?.(idle)
  if (timer !== undefined) clearTimeout(timer)
  idle = undefined
  timer = undefined
}

export function preloadMemberViews(): void {
  stopMemberViewPreload()
  if (saveData()) return
  const current = generation
  // Settings/admin-access is deliberately absent: only member code is warmed.
  const queue: MemberPath[] = ['/home', '/catalog', '/subscription', '/activity', '/settings', '/knowledgebase', '/settings/diy', '/connections', '/affiliates', '/community', '/emby']
  function schedule(): void {
    if (generation !== current || !queue.length) return
    const work = () => {
      if (generation !== current) return
      const paths = queue.splice(0, 2)
      void Promise.allSettled(paths.map(preloadMemberView)).then(schedule)
    }
    if (globalThis.requestIdleCallback) idle = globalThis.requestIdleCallback(work, { timeout: 1500 })
    else timer = setTimeout(work, 100)
  }
  schedule()
}
