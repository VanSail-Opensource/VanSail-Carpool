import { createRouter } from 'vue-router'

import { api } from '@/api/client'
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
    { path: '/connections', name: 'connections', component: () => import('@/views/ConnectionsView.vue') },
    { path: '/catalog', name: 'catalog', component: () => import('@/views/CatalogView.vue') },
    { path: '/payment-result', name: 'payment-result', component: () => import('@/views/PaymentResultView.vue'), meta: { immersive: true, browserPublic: true } },
    { path: '/balance', redirect: { name: 'home', query: { topUp: '1' } } },
    { path: '/activity', name: 'activity', component: () => import('@/views/ActivityView.vue') },
    { path: '/games', redirect: '/activity' },
    { path: '/statistics', name: 'statistics', component: () => import('@/views/StatisticsView.vue') },
    { path: '/affiliates', name: 'affiliates', component: () => import('@/views/AffiliatesView.vue') },
    { path: '/questionnaire', name: 'questionnaire', component: () => import('@/views/QuestionnaireView.vue') },
    { path: '/community', name: 'community', component: () => import('@/views/CommunityView.vue') },
    { path: '/emby', name: 'emby', component: () => import('@/views/EmbyView.vue') },
    { path: '/abuse-records', name: 'abuse-records', component: () => import('@/views/AbuseRecordsView.vue') },
    { path: '/ip-lookup', name: 'ip-lookup', component: () => import('@/views/IPLookupView.vue') },
    { path: '/admin/ip-lookup', name: 'admin-ip-lookup', component: () => import('@/views/AdminIPLookupView.vue') },
    { path: '/admin', redirect: '/admin/settings' },
    { path: '/admin/emby', redirect: '/admin/users' },
    { path: '/admin/entitlements', redirect: '/admin/users' },
    { path: '/admin/pm', redirect: '/admin/users' },
    {
      path: '/admin/users/:userId',
      name: 'admin-user',
      component: () => import('@/views/AdminUserView.vue'),
      meta: { adminSection: 'users' },
    },
    {
      path: '/admin/:section(settings|usdt|host-connectivity|catalog|activity|affiliates|coupons|questionnaires|onboarding|users|compensation|abuse|backups|database|audit)',
      name: 'admin',
      component: () => import('@/views/AdminView.vue'),
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

let catalogCheckVersion = 0
let telegramStartParamConsumed = false
router.afterEach((to, _from, failure) => {
  completeRouteRecovery(to.fullPath)
  if (failure) return
  const version = ++catalogCheckVersion
  const store = useSessionStore()
  if (to.name !== 'catalog' || !store.user || store.user.role === 'admin' || store.session?.captcha?.required) return
  const userId = store.user.id
  // Render the page snapshot while retaining the live, server-enforced renewal gate.
  void api.getDashboard().then(dashboard => {
    if (version !== catalogCheckVersion || store.user?.id !== userId) return
    if (dashboard.activePurchase?.autoRenewEnabled || dashboard.queuedPurchase?.autoRenewEnabled) {
      void router.replace({ name: 'home', query: { autoRenewBlocked: '1' } })
    }
  }).catch(() => undefined)
})

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
    if (match) return { name: 'admin-user', params: { userId: match[1] } }
  }
  const protectedRedirect = resolveProtectedRoute(to, store.user)
  if (protectedRedirect) return protectedRedirect
  return true
})

export default router
