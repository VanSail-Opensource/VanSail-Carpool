<script setup lang="ts">
import { computed, nextTick, shallowRef, useTemplateRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { AnimatePresence, motion } from 'motion-v'

import { useIPLookupAvailability } from '@/composables/useIPLookupAvailability'
import { useCommunityAccess } from '@/composables/useCommunityAccess'
import { useTelegramBackButton } from '@/composables/useTelegramBackButton'
import { useI18n } from '@/i18n'
import { useSessionStore } from '@/stores/session'
import { motionDurations } from '@/composables/motionPresets'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { focusWithoutScrolling } from '@/utils/dom'
import { isTelegramWebAppDetected, telegramFullscreenState } from '@/utils/telegram'
import { usePreferencesStore } from '@/stores/preferences'
import { useTelegramSettingsButton } from '@/composables/useTelegramSettingsButton'
import MobileNavigation from './MobileNavigation.vue'
import SidebarMember from './SidebarMember.vue'
import { desktopNavigationItems } from './navigation'
import { usePageTransition } from './usePageTransition'

const route = useRoute()
const router = useRouter()
const sessionStore = useSessionStore()
const preferences = usePreferencesStore()
useTelegramSettingsButton(() => { void router.push('/settings') })
const { t } = useI18n()
const { activeCombo: hasValidCombo, refresh: refreshCommunityAccess } = useCommunityAccess()

const { enabled: ipLookupEnabled, refresh: refreshIPLookupAvailability } = useIPLookupAvailability()
const desktopItems = computed(() => desktopNavigationItems(t, sessionStore.isAdmin, hasValidCombo.value && preferences.showAroundTX, hasValidCombo.value && preferences.showActivity, ipLookupEnabled.value))
const transition = usePageTransition(router, () => desktopItems.value)
const { reducedMotion, offset } = useMotionPreferences()
const showBackButton = computed(() => !['/', '/home'].includes(route.path))
const appContent = useTemplateRef<globalThis.HTMLDivElement>('appContent')
const mainContent = useTemplateRef<globalThis.HTMLElement>('mainContent')
const isFullscreen = telegramFullscreenState()
const greetingName = computed(() => t('nav.fullscreenGreeting', {
  name: sessionStore.user?.firstName?.trim() || t('nav.memberFallback'),
}))
const greetingUsername = computed(() => sessionStore.user?.telegramUsername?.trim() || '')
const firstPage = shallowRef(true)
const pageMotion = computed(() => {
  const distance = offset(firstPage.value ? 8 : 14)
  const axis = firstPage.value || transition.wide.value ? 'y' : 'x'
  const direction = transition.direction.value === 'backward' ? -1 : 1
  const enter = { opacity: reducedMotion.value ? 0 : 0.75, [axis]: direction * distance }
  const exit = { opacity: 0, [axis]: direction * -distance }
  return {
    initial: reducedMotion.value ? { opacity: 1, x: 0, y: 0 } : enter,
    animate: { opacity: 1, [axis]: 0 },
    exit: reducedMotion.value ? { opacity: 1 } : exit,
    transition: { duration: reducedMotion.value ? 0 : firstPage.value ? 0.5 : motionDurations.normal, ease: [0.23, 1, 0.32, 1] as [number, number, number, number] },
  }
})

function resolveDashboardPersistence(): boolean {
  if (isTelegramWebAppDetected()) return false

  try {
    const storage = globalThis.localStorage
    const probeKey = '__txc_dashboard_storage_probe__'
    storage.setItem(probeKey, '1')
    storage.removeItem(probeKey)
    return true
  } catch {
    return false
  }
}

const dashboardPersistent = resolveDashboardPersistence()

function goBack(): void {
  try {
    void Promise.resolve(router.back()).catch(() => undefined)
  } catch {
    // The router may already be disposing the current view.
  }
}

useTelegramBackButton(showBackButton, goBack)

watch(() => route.path, (_next, previous) => {
  if (!previous) return
  firstPage.value = false
  void refreshCommunityAccess()
  void refreshIPLookupAvailability()
  void preferences.refresh()
  void nextTick()
    .then(() => {
      if (appContent.value) {
        appContent.value.scrollTop = 0
        appContent.value.scrollLeft = 0
      }
      if (mainContent.value) focusWithoutScrolling(mainContent.value)
    })
    .catch(() => undefined)
})
</script>

<template>
  <div class="app-frame" :class="{ 'app-frame--fullscreen': isFullscreen }">
    <UDashboardGroup class="app-dashboard" storage="local" :persistent="dashboardPersistent" storage-key="tx-carpool-shell" unit="rem">
      <UDashboardSidebar id="navigation-compact" class="side-rail app-dashboard__sidebar" :default-size="13" :min-size="13" :max-size="20" :ui="{ body: 'px-0', footer: 'px-0' }" resizable>
        <template #default>
          <nav class="side-rail__nav" :aria-label="$t('nav.primary')">
            <UNavigationMenu :items="desktopItems" orientation="vertical" color="primary" variant="pill" />
          </nav>
        </template>
        <template #footer>
          <footer class="side-rail__footer">
            <SidebarMember />
          </footer>
        </template>
      </UDashboardSidebar>

      <div class="app-route-viewport">
        <AnimatePresence :initial="!reducedMotion" mode="sync">
          <motion.div
            :key="route.fullPath"
            ref="appContent"
            class="app-frame__content"
            :initial="pageMotion.initial"
            :animate="pageMotion.animate"
            :exit="pageMotion.exit"
            :transition="pageMotion.transition"
          >
            <div v-if="isFullscreen" class="app-greeting" role="status">
              <strong>{{ greetingName }}</strong>
              <span v-if="greetingUsername">@{{ greetingUsername }}</span>
            </div>
            <main id="main-content" ref="mainContent" class="app-main" tabindex="-1">
              <slot />
            </main>
          </motion.div>
        </AnimatePresence>
      </div>
    </UDashboardGroup>

    <MobileNavigation :is-admin="sessionStore.isAdmin" />
  </div>
</template>
