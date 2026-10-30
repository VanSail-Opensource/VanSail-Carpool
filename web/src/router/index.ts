import { createRouter } from 'vue-router'

import { useSessionStore } from '@/stores/session'
import { getTelegramStartParam, isTelegramWebAppDetected } from '@/utils/telegram'
import { resolveProtectedRoute } from './guards'
import { createAppHistory } from './history'
import { beginRouteRecovery, completeRouteRecovery, isRouteChunkError } from './recovery'

const router = createRouter({
  history: createAppHistory(),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    { path: '/', name: 'root', component: () => import('@/views/EntryView.vue') },
    {
      path: '/onboarding',
      name: 'onboarding',
      component: () => import('@/views/OnboardingView.vue'),
      meta: { immersive: true },
    },
    { path: '/home', name: 'home', component: () => import('@/views/HomeView.vue') },
    { path: '/subscription', name: 'subscription', component: () => import('@/views/SubscriptionView.vue') },
    { path: '/settings', name: 'settings', component: () => import('@/views/SettingsView.vue') },
    { path: '/settings/diy', name: 'diy', component: () => import('@/views/DIYView.vue') },
    { path: '/settings/admin-access', name: 'admin-access', component: () => import('@/components/settings/AdminAccessPanel.vue') },
    { path: '/connections', name: 'connections', component: () => import('@/views/ConnectionsView.vue') },
    { path: '/catalog', name: 'catalog', component: () => import('@/views/CatalogView.vue') },
    { path: '/payment-result', name: 'payment-result', component: () => import('@/views/PaymentResultView.vue'), meta: { immersive: true, browserPublic: true } },
    { path: '/balance', redirect: { name: 'home', query: { topUp: '1' } } },
    { path: '/activity', name: 'activity', component: () => import('@/views/ActivityView.vue') },
    { path: '/games', redirect: '/activity' },
    { path: '/affiliates', name: 'affiliates', component: () => import('@/views/AffiliatesView.vue') },
    { path: '/community', name: 'community', component: () => import('@/views/CommunityView.vue') },
    { path: '/emby', name: 'emby', component: () => import('@/views/EmbyView.vue') },
    { path: '/ip-lookup', name: 'ip-lookup', component: () => import('@/views/IPLookupView.vue') },
    { path: '/admin/:pathMatch(.*)*', redirect: to => ({ name: 'admin-access', query: { returnTo: to.fullPath } }) },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

let telegramStartParamConsumed = false
router.afterEach((to) => { completeRouteRecovery(to.fullPath) })

router.onError((error, to) => {
  if (!isRouteChunkError(error) || !beginRouteRecovery(to.fullPath)) return
  if (isTelegramWebAppDetected()) {
    void router.replace(to.fullPath).catch(() => undefined)
    return
  }
  try {
    window.location.reload()
  } catch {
    // A WebView may reject reload while its host is changing state.
  }
})

router.beforeEach(async (to) => {
  const store = useSessionStore()
  if (to.meta.browserPublic === true && !isTelegramWebAppDetected()) return true
  await store.bootstrap()
  if (store.status === 'error') return true
  if (!telegramStartParamConsumed && to.name === 'root' && store.user) {
    telegramStartParamConsumed = true
    const match = /^admin_user_([A-Za-z0-9_-]{1,64})$/.exec(getTelegramStartParam() ?? '')
    if (match && store.isAdmin) return { name: 'admin-access', query: { returnTo: `/admin/users/${match[1]}` } }
  }
  const protectedRedirect = resolveProtectedRoute(to, store.user)
  if (protectedRedirect) return protectedRedirect
  return true
})

export default router
