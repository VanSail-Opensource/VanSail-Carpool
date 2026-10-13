<script setup lang="ts">
import { computed, nextTick, useTemplateRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { localizedError, useI18n } from '@/i18n'
import { useSessionStore } from '@/stores/session'
import { useBrowserSession } from './session'
import { adminGroups, consoleNavigation } from './navigation'
import BrowserPreferences from './BrowserPreferences.vue'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const identity = useSessionStore()
const session = useBrowserSession()
const items = computed(() => consoleNavigation(t))
const section = computed(() => String(route.params.section || route.meta.adminSection || 'settings'))
const group = computed(() => adminGroups.find(value => value.sections.some(item => item.value === section.value)))
const pageTitle = computed(() => t(group.value?.sections.find(item => item.value === section.value)?.labelKey ?? 'browser.console'))
const mainContent = useTemplateRef<globalThis.HTMLElement>('mainContent')
watch(() => route.fullPath, () => { void nextTick(() => {
  mainContent.value?.parentElement?.scrollTo({ top: 0, left: 0 })
  if (!mainContent.value?.closest('[aria-hidden="true"], [inert]')) mainContent.value?.focus({ preventScroll: true })
}) })
const breadcrumbs = computed(() => [{ label: t('browser.console'), to: '/admin/settings' }, { label: t(group.value?.labelKey ?? 'adminNav.system') }])
const name = computed(() => identity.user?.firstName || identity.user?.telegramUsername || t('browser.administrator'))
async function logout(): Promise<void> {
  try { await session.logout(); await router.replace('/') }
  catch (caught) { session.error = localizedError(caught, 'browser.logoutFailed') }
}
</script>

<template>
  <UDashboardGroup class="browser-console" storage="local" storage-key="txc-admin-workspace" unit="px">
    <UDashboardSidebar id="admin-navigation" :default-size="240" :min-size="208" :max-size="320" :collapsed-size="64" collapsible resizable :ui="{ header: 'browser-sidebar-header', body: 'browser-sidebar-body', footer: 'browser-sidebar-footer' }">
      <template #header="{ collapsed }">
        <RouterLink class="browser-wordmark" to="/admin/settings" :aria-label="t('app.name')">
          <UIcon name="i-ph-intersect-square-bold" class="browser-brand-icon" />
          <span v-if="!collapsed">{{ t('app.name') }}<small>{{ t('browser.console') }}</small></span>
        </RouterLink>
      </template>
      <template #default="{ collapsed }">
        <nav :aria-label="t('nav.primary')"><UNavigationMenu :collapsed="collapsed" :items="items" orientation="vertical" color="neutral" variant="pill" /></nav>
      </template>
      <template #footer="{ collapsed }">
        <div class="browser-account">
          <UIcon name="i-ph-user-circle-bold" />
          <span v-if="!collapsed"><strong>{{ name }}</strong><small>{{ t('browser.administrator') }}</small></span>
          <UButton icon="i-ph-sign-out-bold" color="neutral" variant="ghost" :aria-label="t('browser.logout')" @click="logout" />
        </div>
      </template>
    </UDashboardSidebar>
    <UDashboardPanel id="admin-workspace" :ui="{ body: 'browser-workspace' }">
      <template #header>
        <UDashboardNavbar :ui="{ root: 'browser-navbar' }">
          <template #left><UDashboardSidebarCollapse /><UBreadcrumb :items="breadcrumbs" /></template>
          <template #right><BrowserPreferences /></template>
        </UDashboardNavbar>
      </template>
      <template #body><main id="main-content" ref="mainContent" tabindex="-1"><h1 class="sr-only">{{ pageTitle }}</h1><UAlert v-if="session.error" color="warning" variant="soft" :title="session.error" class="mb-4" /><slot /></main></template>
    </UDashboardPanel>
  </UDashboardGroup>
</template>
