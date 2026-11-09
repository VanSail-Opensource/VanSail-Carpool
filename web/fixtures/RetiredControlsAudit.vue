<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { en } from '@nuxt/ui/locale'
import type { AdminUserDetail } from '@/api/adminOperations'
import UsagePanel from '@/components/dashboard/UsagePanel.vue'
import type { UsageStatistics } from '@/api/types'
import type { SquadProduct } from '@/api/types'
import AdminSectionState from '@/components/admin/AdminSectionState.vue'
import AdminUserProviderActions from '@/components/admin/users/AdminUserProviderActions.vue'
import AdminSquadEditor from '@/components/admin/AdminSquadEditor.vue'
import StatisticsNodes from '@/components/statistics/StatisticsNodes.vue'

const params = new globalThis.URLSearchParams(globalThis.location.search)
const usage = params.has('usage')
const statistics: UsageStatistics = { usedTrafficBytes: '26843545600', lifetimeTrafficBytes: '214748364800', trafficLimitBytes: '107374182400', onlineAt: null, lastTrafficResetAt: '2026-11-01T00:00:00Z', categories: [], sparklineData: [], topNodes: [{ uuid: 'Tokyo', name: 'Tokyo', totalBytes: '26843545600' }] }
const loading = shallowRef(true)
const error = shallowRef<string | null>(null)
const editor = shallowRef(false)
const node = { uuid: '373f14bc-089a-4c3a-91c3-3421e7c83367', name: 'Tokyo transit', countryCode: 'JP', consumptionMultiplier: 1 as const, providerName: null }
const nodes = params.has('empty') ? [] : [node]
const detail = { temporaryBan: null, synchronization: { remoteUserId: '42' } } as AdminUserDetail
const squad = { id: 'squad', groupId: node.uuid, name: 'Tokyo transit', description: '', price: { minor: '100', currency: 'TXB', scale: 2 }, visible: true, stockLimit: null, activationRequired: false, profile: null } as unknown as SquadProduct
async function load() {
  loading.value = true
  if (params.has('slow')) await new Promise(resolve => globalThis.setTimeout(resolve, 1500))
  error.value = params.has('error') && !error.value ? 'Constructed unavailable state' : null
  loading.value = false
}
onMounted(load)
</script>

<template>
  <UApp :locale="en">
    <main class="p-5 mx-auto max-w-6xl grid gap-6 min-w-0">
      <AdminSectionState :loading="loading" :error="error" @retry="load">
        <UsagePanel v-if="usage && !params.has('empty')" :statistics="statistics" :ratio="0.25" :stale="params.has('stale')" fetched-at="2026-11-08T00:00:00Z" />
        <template v-if="!usage">
          <StatisticsNodes :snapshot="{ generatedAt: '2026-10-16T00:00:00Z', stale: false, nodes: nodes.map(item => ({ ...item, online: true, usersOnline: 3, rxBytesPerSec: '1048576', txBytesPerSec: '524288', xrayVersion: '26.10', multiplier: 1 as const })) }" :loading="false" />
          <AdminUserProviderActions user-id="member" :detail="detail" />
          <UButton :label="$t('common.edit')" @click="editor = true" />
          <AdminSquadEditor v-if="editor" :squad="squad" :busy="false" @cancel="editor = false" />
        </template>
      </AdminSectionState>
    </main>
  </UApp>
</template>
