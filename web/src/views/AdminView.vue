<script setup lang="ts">
import type { Component } from 'vue'
import { computed, defineAsyncComponent } from 'vue'
import { useRoute } from 'vue-router'

import AdminSettingsPanel from '@/components/admin/AdminSettingsPanel.vue'

const route = useRoute()
const panels: Record<string, Component> = {
  settings: AdminSettingsPanel,
  'api-access': defineAsyncComponent(() => import('@/components/admin/api-access/AdminAPITokens.vue')),
  xnode: defineAsyncComponent(() => import('@/components/admin/xnode/XNodeWorkspace.vue')),
  usdt: defineAsyncComponent(() => import('@/components/admin/usdt/AdminUSDTPage.vue')),
  'host-connectivity': defineAsyncComponent(() => import('@/components/admin/connectivity/AdminConnectivitySettings.vue')),
  catalog: defineAsyncComponent(() => import('@/components/admin/AdminCatalogPanel.vue')),
  activity: defineAsyncComponent(() => import('@/components/admin/AdminActivityPanel.vue')),
  affiliates: defineAsyncComponent(() => import('@/components/admin/affiliates/AdminAffiliatesPanel.vue')),
  coupons: defineAsyncComponent(() => import('@/components/admin/AdminCouponsPanel.vue')),
  questionnaires: defineAsyncComponent(() => import('@/components/admin/AdminQuestionnairesPanel.vue')),
  onboarding: defineAsyncComponent(() => import('@/components/admin/AdminOnboardingPanel.vue')),
  users: defineAsyncComponent(() => import('@/components/admin/AdminUsersPanel.vue')),
  compensation: defineAsyncComponent(() => import('@/components/admin/AdminCompensationPanel.vue')),
  abuse: defineAsyncComponent(() => import('@/components/admin/AdminAbusePanel.vue')),
  backups: defineAsyncComponent(() => import('@/components/admin/AdminBackupsPanel.vue')),
  database: defineAsyncComponent(() => import('@/components/admin/AdminDatabasePanel.vue')),
  audit: defineAsyncComponent(() => import('@/components/admin/AdminAuditPanel.vue')),
}
const panel = computed(() => panels[String(route.params.section)] ?? AdminSettingsPanel)
</script>

<template>
  <component :is="panel" :key="String(route.params.section)" />
</template>
