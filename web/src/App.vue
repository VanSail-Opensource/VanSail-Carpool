<script setup lang="ts">
import { computed, onMounted, onUnmounted, shallowRef, watch } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import { en, zh_cn } from '@nuxt/ui/locale'

import AppShell from '@/components/layout/AppShell.vue'
import AuthGate from '@/components/session/AuthGate.vue'
import CaptchaGate from '@/components/session/CaptchaGate.vue'
import MemberSessionStatus from '@/components/session/MemberSessionStatus.vue'
import AppErrorBoundary from '@/components/session/AppErrorBoundary.vue'
import { useSessionStore } from '@/stores/session'
import { usePreferencesStore } from '@/stores/preferences'
import { useDisplayCurrency } from '@/composables/useDisplayCurrency'
import { useI18n } from '@/i18n'
import { isTelegramWebAppDetected } from '@/utils/telegram'
import { onboardingRequiredEvent } from '@/api/http'

const route = useRoute()
const router = useRouter()
const sessionStore = useSessionStore()
const preferences = usePreferencesStore()
const displayCurrency = useDisplayCurrency()
const immersive = computed(() => route.meta.immersive === true)
const browserPublic = computed(() => route.meta.browserPublic === true && !isTelegramWebAppDetected())
const { locale } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
const retryingRoute = shallowRef(false)
const routeReady = computed(() => sessionStore.isAuthenticated && !retryingRoute.value && route.matched.length > 0)
const captcha = computed(() => sessionStore.session?.captcha?.required ? sessionStore.session.captcha : undefined)
const activeMember = computed(() => routeReady.value && !captcha.value ? sessionStore.user?.id : undefined)
const shellRevision = shallowRef(0)
watch(() => sessionStore.user?.id, (_id, previous) => {
  if (previous) shellRevision.value++
}, { flush: 'sync' })
watch(() => sessionStore.onboardingComplete ? activeMember.value ?? null : null, id => preferences.bind(id), { immediate: true })

watch(activeMember, userID => {
  displayCurrency.hydrate(userID ? sessionStore.user?.displayCurrency : undefined)
  if (userID) void displayCurrency.refresh()
}, { immediate: true })

async function retrySession(): Promise<void> {
  const destination = router.options.history.location || '/'
  retryingRoute.value = true
  try {
    await sessionStore.bootstrap(true)
    if (sessionStore.isAuthenticated) {
      const target = router.resolve(destination)
      await router.replace({ path: target.path, query: target.query, hash: target.hash, force: true })
    }
  } finally {
    retryingRoute.value = false
  }
}

let refreshingOnboarding = false
async function refreshRequiredOnboarding(): Promise<void> {
  if (refreshingOnboarding) return
  refreshingOnboarding = true
  try {
    await sessionStore.bootstrap(true)
    if (sessionStore.user && sessionStore.user.onboardingState !== 'complete') {
      await router.replace('/onboarding')
    }
  } finally {
    refreshingOnboarding = false
  }
}
function onOnboardingRequired(): void {
  void refreshRequiredOnboarding().catch(() => {
    // The route guard will retry after the next navigation or Mini App open.
  })
}
onMounted(() => globalThis.addEventListener(onboardingRequiredEvent, onOnboardingRequired))
onUnmounted(() => globalThis.removeEventListener(onboardingRequiredEvent, onOnboardingRequired))
</script>

<template>
  <UApp :locale="uiLocale" :toaster="{ position: 'top-center' }">
    <AppErrorBoundary>
      <RouterView v-if="browserPublic" v-slot="{ Component, route: currentRoute }">
        <component :is="Component" :key="currentRoute.fullPath" />
      </RouterView>
      <AuthGate
        v-else-if="sessionStore.status === 'error'"
        :message="sessionStore.error ?? $t('auth.authenticationFailed')"
        :code="sessionStore.errorCode ?? undefined"
        @retry="retrySession"
      />
      <CaptchaGate v-else-if="captcha" :key="sessionStore.user?.id" :challenge="captcha" @verified="sessionStore.updateSession" />
      <RouterView v-else-if="routeReady && immersive" v-slot="{ Component, route: currentRoute }">
        <component :is="Component" :key="`${sessionStore.user?.id ?? ''}:${currentRoute.fullPath}`" />
      </RouterView>
      <AppShell v-else :key="shellRevision" :authorized="routeReady">
        <template #pending><MemberSessionStatus :authenticated="sessionStore.isAuthenticated" /></template>
        <RouterView v-slot="{ Component, route: currentRoute }">
          <component :is="Component" :key="currentRoute.fullPath" />
        </RouterView>
      </AppShell>
    </AppErrorBoundary>
  </UApp>
</template>
