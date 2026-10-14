<script setup lang="ts">
import { onMounted } from 'vue'
import BrowserLanding from '@/browser/BrowserLanding.vue'
import BrowserAccess from '@/browser/BrowserAccess.vue'
import AdminAccessPanel from '@/components/settings/AdminAccessPanel.vue'
import { useBrowserSession } from '@/browser/session'

const params = new globalThis.URLSearchParams(globalThis.location.search)
const session = useBrowserSession()
onMounted(() => { if (!params.has('link') && !params.has('exchange')) void session.bootstrap() })
</script>

<template>
  <UApp>
    <AdminAccessPanel v-if="params.has('link')" />
    <BrowserAccess v-else-if="params.has('exchange')" />
    <BrowserLanding v-else />
  </UApp>
</template>
