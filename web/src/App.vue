<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { RouterView, useRoute, useRouter } from 'vue-router'
import { en, zh_cn } from '@nuxt/ui/locale'

import AppShell from '@/components/layout/AppShell.vue'
import AuthGate from '@/components/session/AuthGate.vue'
import CaptchaGate from '@/components/session/CaptchaGate.vue'
import SessionEntrance from '@/components/session/SessionEntrance.vue'
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
watch(() => sessionStore.onboardingComplete ? sessionStore.user?.id ?? null : null, id => preferences.bind(id), { immediate: true })

watch(() => sessionStore.user?.id, (userID) => {
  if (!userID) return
  displayCurrency.hydrate(sessionStore.user?.displayCurrency)
  void displayCurrency.refresh()
}, { immediate: true })

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
      <SessionEntrance v-else :status="sessionStore.status" :authenticated="sessionStore.isAuthenticated" :route-ready="route.matched.length > 0 || sessionStore.session?.captcha?.required === true">
        <AuthGate
          v-if="sessionStore.status === 'error'"
          :message="sessionStore.error ?? $t('auth.authenticationFailed')"
          @retry="sessionStore.bootstrap(true)"
        />
        <CaptchaGate v-else-if="sessionStore.session?.captcha?.required" :challenge="sessionStore.session.captcha" @verified="sessionStore.updateSession" />
        <template v-else>
          <RouterView v-if="immersive" v-slot="{ Component, route: currentRoute }">
            <component :is="Component" :key="currentRoute.fullPath" />
          </RouterView>
          <AppShell v-else>
            <RouterView v-slot="{ Component, route: currentRoute }">
              <component :is="Component" :key="currentRoute.fullPath" />
            </RouterView>
          </AppShell>
        </template>
      </SessionEntrance>
    </AppErrorBoundary>
  </UApp>
</template>
