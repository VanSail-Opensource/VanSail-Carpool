import { createRouter, createWebHistory } from 'vue-router'
import { useBrowserSession } from './session'

const router = createRouter({
  history: createWebHistory(),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    { path: '/', name: 'browser-entry', component: () => import('./BrowserLanding.vue'), meta: { public: true } },
    { path: '/admin/login', redirect: to => ({ path: '/', query: to.query }) },
    { path: '/admin/access', name: 'browser-access', component: () => import('./BrowserAccess.vue'), meta: { public: true } },
    { path: '/payment-result', name: 'payment-result', component: () => import('@/views/PaymentResultView.vue'), meta: { public: true, payment: true } },
    { path: '/admin', redirect: '/admin/settings' },
    { path: '/admin/emby', redirect: '/admin/users' },
    { path: '/admin/entitlements', redirect: '/admin/users' },
    { path: '/admin/pm', redirect: '/admin/users' },
    { path: '/admin/users/:userId', name: 'admin-user', component: () => import('@/views/AdminUserView.vue'), meta: { console: true, adminSection: 'users' } },
    { path: '/admin/ip-lookup', name: 'admin-ip-lookup', component: () => import('@/views/AdminIPLookupView.vue'), meta: { console: true, adminSection: 'ip-lookup' } },
    { path: '/admin/:section(settings|api-access|xnode|usdt|host-connectivity|catalog|activity|affiliates|coupons|questionnaires|onboarding|users|compensation|abuse|backups|database|audit)', name: 'admin', component: () => import('@/views/AdminView.vue'), meta: { console: true } },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async to => {
  if (to.name === 'browser-access' || to.meta.payment) return true
  const session = useBrowserSession()
  await session.bootstrap()
  if (to.meta.console && !session.isAdmin) return { path: '/', query: { returnTo: to.fullPath } }
  if (to.name === 'browser-entry' && session.isAdmin) return '/admin/settings'
  return true
})
router.onError(() => window.location.reload())
export default router
