<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import type { AdminUserDetail } from '@/api/adminOperations'
import BrowserConsole from '@/browser/BrowserConsole.vue'
import AdminUserAccountContext from '@/components/admin/users/AdminUserAccountContext.vue'
import ComingSoonLinks from '@/components/dashboard/ComingSoonLinks.vue'
import { desktopNavigationItems } from '@/components/layout/navigation'
import { usePreferencesStore } from '@/stores/preferences'
import { useI18n } from '@/i18n'

const { t, locale } = useI18n()
const params = new globalThis.URLSearchParams(globalThis.location.search)
const member = params.has('member')
usePreferencesStore().value = { activeCombo: true, showAroundTx: true, showActivity: false, includeNodePrices: true, showReferralUsername: true, notifications: { combos: true, traffic: true, money: true, activity: true, account: true } }
const loading = shallowRef(true)
const failed = shallowRef(false)
let attempts = 0
const detail = computed(() => ({ affiliateHistory: { items: params.has('empty') ? [] : [
  { firstName: 'Lin', lastName: 'Wang', registeredAt: '2026-10-15T00:00:00Z', commissionAmount: { minor: '250', currency: 'TXB', scale: 2 }, status: 'successful' },
] } }) as AdminUserDetail)
async function load(): Promise<void> {
  loading.value = true; failed.value = false
  if (params.has('slow')) await new Promise(resolve => globalThis.setTimeout(resolve, 2000))
  failed.value = params.has('error') && attempts++ === 0
  loading.value = false
}
onMounted(() => { void load() })
</script>

<template>
  <UApp :locale="locale === 'zh-CN' ? zh_cn : en">
    <main v-if="member" id="main-content" class="page page--home">
      <UNavigationMenu class="hidden min-[900px]:block" :items="desktopNavigationItems(t, false, true)" orientation="vertical" />
      <ComingSoonLinks :has-valid-combo="true" />
    </main>
    <BrowserConsole v-else>
      <UAlert v-if="loading" icon="i-ph-circle-notch" :title="t('adminUserProfile.loading')" />
      <UAlert v-else-if="failed" color="warning" :title="t('adminUserProfile.unavailable')" :actions="[{ label: t('common.refresh'), onClick: load }]" />
      <AdminUserAccountContext v-else :detail="detail" />
    </BrowserConsole>
  </UApp>
</template>
