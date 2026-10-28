<script setup lang="ts">
import { computed, nextTick, onMounted, onScopeDispose, onUnmounted, watch } from 'vue'
import { useI18n } from '@/i18n'
import BrowserPreferences from './BrowserPreferences.vue'
import { useBrowserSession } from './session'

const { t } = useI18n()
const session = useBrowserSession()
const member = computed(() => session.state?.kind === 'member' ? session.state.user : null)
const memberName = computed(() => member.value?.firstName || member.value?.telegramUsername || t('nav.member'))
const miniAppUrl = computed(() => session.config?.miniAppUrl || (session.state && 'miniAppUrl' in session.state ? session.state.miniAppUrl : ''))
let redirectTimer: ReturnType<typeof globalThis.setTimeout> | undefined
watch([member, miniAppUrl], ([user, url]) => {
  if (redirectTimer) globalThis.clearTimeout(redirectTimer)
  if (user && url) redirectTimer = globalThis.setTimeout(() => globalThis.location.replace(url), 2000)
}, { immediate: true })
onScopeDispose(() => { if (redirectTimer) globalThis.clearTimeout(redirectTimer) })
let cleanupReveals = () => {}
function installReveals() {
  cleanupReveals()
  const reduced = globalThis.matchMedia('(prefers-reduced-motion: reduce)')
  const animations: globalThis.Animation[] = []
  if (!globalThis.IntersectionObserver) return
  const observer = new globalThis.IntersectionObserver(entries => {
    for (const entry of entries) {
      if (!entry.isIntersecting) continue
      observer.unobserve(entry.target)
      if (!reduced.matches && entry.boundingClientRect.top >= globalThis.innerHeight) {
        const offset = entry.target.classList.contains('landing-services') ? 'translateX(-12px)' : 'translateY(12px)'
        animations.push(entry.target.animate([{ transform: offset }, { transform: 'none' }], { duration: 600, easing: 'ease-out' }))
      }
    }
  }, { rootMargin: '0px 0px 96px 0px' })
  globalThis.document.querySelectorAll('.landing-services, .browser-how').forEach(element => {
    if (element.getBoundingClientRect().top >= globalThis.innerHeight) observer.observe(element)
  })
  const stop = () => { observer.disconnect(); animations.forEach(animation => animation.cancel()) }
  reduced.addEventListener('change', stop)
  cleanupReveals = () => { stop(); reduced.removeEventListener('change', stop) }
}
onMounted(installReveals)
watch(member, () => { void nextTick(installReveals) })
onUnmounted(() => cleanupReveals())
</script>

<template>
  <div class="browser-landing">
    <header class="browser-landing-header">
      <RouterLink class="browser-wordmark" to="/"><UIcon name="i-ph-intersect-square-bold" class="browser-brand-icon" /><span>{{ t('app.name') }}</span></RouterLink>
      <BrowserPreferences />
    </header>
    <main>
      <section v-if="member" class="browser-member" role="status">
        <UIcon name="i-ph-check-circle-bold" class="browser-access-icon" />
        <h1>{{ t('browser.memberTitle', { name: memberName }) }}</h1>
        <p>{{ t('browser.memberHint') }}</p>
        <UButton :to="miniAppUrl" external :label="t('browser.memberEntrance')" icon="i-ph-arrow-up-right-bold" />
      </section>
      <template v-else>
        <section class="browser-hero">
          <div class="landing-display" aria-hidden="true"><img src="/landing/hero-connectivity.png" alt="" /><span>VanSail</span></div>
          <div class="browser-hero-copy">
            <p class="browser-intro">{{ t('browser.landing.eyebrow') }}</p>
            <h1>{{ t('browser.landing.title') }}</h1>
            <p class="browser-hero-description">{{ t('browser.landing.description') }}</p>
            <div class="browser-entrances">
              <UButton :to="miniAppUrl || undefined" external :disabled="!miniAppUrl" size="lg" :label="t('browser.memberEntrance')" icon="i-ph-arrow-up-right-bold" />
              <UButton :to="miniAppUrl || undefined" external :disabled="!miniAppUrl" color="neutral" variant="outline" size="lg" :label="t('browser.adminEntrance')" icon="i-ph-sign-in-bold" />
            </div>
            <UAlert v-if="session.error" color="warning" variant="soft" :title="session.error" class="browser-entry-notice">
              <template #actions><UButton v-if="session.error" variant="link" :label="t('common.tryAgain')" @click="session.bootstrap(true)" /></template>
            </UAlert>
            <p class="browser-entry-hint">{{ t('browser.accessInstructions') }}</p>
          </div>
        </section>
        <section class="landing-services" :aria-label="t('browser.landing.services')">
          <img class="landing-service-image" src="/landing/service-overview.png" alt="" loading="lazy" />
          <aside class="browser-service-map" :aria-label="t('browser.landing.services')">
            <div class="browser-service-map-header"><UIcon name="i-ph-circles-three-plus-bold" /><span>{{ t('browser.landing.services') }}</span></div>
            <div class="browser-service"><UIcon name="i-ph-globe-bold" /><div><strong>{{ t('browser.landing.connectivity') }}</strong><span>{{ t('browser.landing.connectivityHint') }}</span></div></div>
            <div class="browser-service"><UIcon name="i-ph-monitor-play-bold" /><div><strong>{{ t('browser.landing.streaming') }}</strong><span>{{ t('browser.landing.streamingHint') }}</span></div></div>
            <div class="browser-service"><UIcon name="i-ph-users-three-bold" /><div><strong>{{ t('browser.landing.membership') }}</strong><span>{{ t('browser.landing.membershipHint') }}</span></div></div>
            <div class="browser-service-map-footer"><UIcon name="i-ph-paper-plane-tilt-bold" /><span>{{ t('browser.landing.telegram') }}</span></div>
          </aside>
        </section>
        <section class="browser-how" :aria-label="t('browser.landing.how')">
          <h2>{{ t('browser.landing.how') }}</h2>
          <img class="landing-how-image" src="/landing/membership-management.png" alt="" loading="lazy" />
          <div class="browser-how-row"><UIcon name="i-ph-package-bold" /><strong>{{ t('browser.landing.plans') }}</strong><p>{{ t('browser.landing.plansHint') }}</p></div>
          <div class="browser-how-row"><UIcon name="i-ph-wallet-bold" /><strong>{{ t('browser.landing.wallet') }}</strong><p>{{ t('browser.landing.walletHint') }}</p></div>
          <div class="browser-how-row"><UIcon name="i-ph-arrows-clockwise-bold" /><strong>{{ t('browser.landing.renewals') }}</strong><p>{{ t('browser.landing.renewalsHint') }}</p></div>
        </section>
      </template>
    </main>
    <footer class="browser-landing-footer"><img src="/landing/footer-connectivity.png" alt="" loading="lazy" /><span>{{ t('app.name') }}</span><span>{{ t('browser.landing.footer') }}</span></footer>
  </div>
</template>
