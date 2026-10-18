<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import AdminUserMetrics from '@/components/admin/users/AdminUserMetrics.vue'
import AdminUserScan from '@/components/admin/users/AdminUserScan.vue'
import AdminUserBalanceAdjustment from '@/components/admin/users/AdminUserBalanceAdjustment.vue'
import XNodeMetrics from '@/components/admin/xnode/XNodeMetrics.vue'
import StatisticsPage from '@/components/statistics/StatisticsPage.vue'
import type { XNodeMetrics as Metrics } from '@/api/xnode'
import { useI18n } from '@/i18n'
defineProps<{ machine: Metrics; view: string }>()
const { locale } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
const userId = shallowRef('member')
Object.assign(globalThis, { __nativeAdminTarget: (id: string) => { userId.value = id } })
</script>
<template>
  <UApp :locale="uiLocale"><main class="native-audit-shell"><AdminUserScan v-if="view === 'scan'" :user-id="userId" /><StatisticsPage v-else-if="view === 'statistics'" /><XNodeMetrics v-else-if="view === 'machine'" :metrics="machine" /><template v-else><AdminUserBalanceAdjustment :user-id="userId" /><AdminUserMetrics :user-id="userId" /></template></main></UApp>
</template>
<style scoped>
.native-audit-shell { max-width: 1200px; margin: auto; padding: 24px; display: grid; gap: 20px; min-width: 0; }
@media(max-width:600px) { .native-audit-shell { padding: 16px; } }
</style>
