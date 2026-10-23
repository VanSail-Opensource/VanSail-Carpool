<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { en } from '@nuxt/ui/locale'
import type { AdminUserDetail } from '@/api/adminOperations'
import type { SquadProduct } from '@/api/types'
import AdminSectionState from '@/components/admin/AdminSectionState.vue'
import AdminUserProviderActions from '@/components/admin/users/AdminUserProviderActions.vue'
import AdminSquadEditor from '@/components/admin/AdminSquadEditor.vue'
import StatisticsNodes from '@/components/statistics/StatisticsNodes.vue'
import SquadNodeBlocks from '@/components/catalog/SquadNodeBlocks.vue'

const params = new globalThis.URLSearchParams(globalThis.location.search)
const loading = shallowRef(true)
const error = shallowRef<string | null>(null)
const editor = shallowRef(false)
const node = { uuid: '373f14bc-089a-4c3a-91c3-3421e7c83367', name: 'Tokyo transit', countryCode: 'JP', consumptionMultiplier: 1.25, providerName: null }
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
        <SquadNodeBlocks :nodes="nodes" />
        <StatisticsNodes :snapshot="{ generatedAt: '2026-10-16T00:00:00Z', stale: false, nodes: nodes.map(item => ({ ...item, online: true, usersOnline: 3, rxBytesPerSec: '1048576', txBytesPerSec: '524288', xrayVersion: '26.10', multiplier: 1.25 })) }" :loading="false" />
        <AdminUserProviderActions user-id="member" :detail="detail" />
        <UButton :label="$t('common.edit')" @click="editor = true" />
        <AdminSquadEditor v-if="editor" :squad="squad" :busy="false" @cancel="editor = false" />
      </AdminSectionState>
    </main>
  </UApp>
</template>
