<script setup lang="ts">
import { onMounted } from 'vue'
import BrowserLanding from '@/browser/BrowserLanding.vue'
import BrowserAccess from '@/browser/BrowserAccess.vue'
import BrowserConsole from '@/browser/BrowserConsole.vue'
import AdminAccessPanel from '@/components/settings/AdminAccessPanel.vue'
import AuthGate from '@/components/session/AuthGate.vue'
import SessionEntrance from '@/components/session/SessionEntrance.vue'
import { useBrowserSession } from '@/browser/session'
import { useSessionStore } from '@/stores/session'

const params = new globalThis.URLSearchParams(globalThis.location.search)
const session = useBrowserSession()
const member = useSessionStore()
onMounted(() => {
  if (params.has('recovery')) void member.bootstrap()
  else if (!params.has('link') && !params.has('exchange')) void session.bootstrap()
})
</script>

<template>
  <UApp>
    <SessionEntrance v-if="params.has('recovery')" :status="member.status" :authenticated="member.isAuthenticated">
      <AuthGate v-if="member.status === 'error'" :message="member.error ?? ''" :code="member.errorCode ?? undefined" @retry="member.bootstrap(true)" />
    </SessionEntrance>
    <AdminAccessPanel v-else-if="params.has('link')" />
    <BrowserAccess v-else-if="params.has('exchange')" />
    <BrowserConsole v-else-if="params.has('console')"><p>{{ $t('browser.console') }}</p></BrowserConsole>
    <BrowserLanding v-else />
  </UApp>
</template>
