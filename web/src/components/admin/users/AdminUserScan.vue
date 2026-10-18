<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { adminOperationsApi } from '@/api/adminOperations'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n, localizedError } from '@/i18n'
import { createUuid } from '@/utils/browserCompatibility'
import { formatDateTime } from '@/utils/format'
const props = defineProps<{ userId: string }>()
const { t } = useI18n()
const scan = shallowRef<Awaited<ReturnType<typeof adminOperationsApi.requestUserConnections>> | null>(null)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
let timer: ReturnType<typeof globalThis.setTimeout> | undefined
let generation = 0
let active = true
let requestKey: string | undefined
const rows = computed(() => scan.value?.nodes.flatMap(node => node.unavailable ? [{ name: node.name, ip: t('adminNative.unavailable'), lastSeen: '' }] : node.ips.map(ip => ({ name: node.name, ip: ip.ip, lastSeen: ip.lastSeen }))) ?? [])
const columns = computed(() => [{ accessorKey: 'name', header: t('adminWorkspace.name') }, { accessorKey: 'ip', header: t('adminWorkspace.identity') }, { id: 'date', header: t('adminWorkspace.updated') }])
function reset(): void { generation++; if (timer) globalThis.clearTimeout(timer); scan.value = null; error.value = null; busy.value = false; requestKey = undefined }
function schedule(ticket: number, id: string): void {
  if (!scan.value || scan.value.isCompleted || scan.value.isFailed || !active || ticket !== generation) { busy.value = false; return }
  timer = globalThis.setTimeout(async () => {
    try {
      const result = await adminOperationsApi.pollUserConnections(id, scan.value!.id)
      if (!active || ticket !== generation) return
      scan.value = result; schedule(ticket, id)
    } catch (caught) { if (active && ticket === generation) { error.value = localizedError(caught, 'errors.adminLoad'); busy.value = false } }
  }, 1200)
}
async function start(): Promise<void> {
  if (busy.value) return
  const ticket = ++generation
  const id = props.userId
  busy.value = true; error.value = null
  try {
    requestKey ??= createUuid()
    const result = await adminOperationsApi.requestUserConnections(id, requestKey)
    if (!active || ticket !== generation) return
    requestKey = undefined; scan.value = result; schedule(ticket, id)
  } catch (caught) { if (active && ticket === generation) { error.value = localizedError(caught, 'errors.adminAction'); busy.value = false } }
}
watch(() => props.userId, reset)
window.addEventListener('txc:session-cleared', reset)
onScopeDispose(() => { active = false; reset(); window.removeEventListener('txc:session-cleared', reset) })
</script>
<template>
  <section class="grid gap-3 min-w-0">
    <div><UButton icon="i-ph-radar" :label="t('adminUserProfile.scanActiveIp')" :loading="busy" :disabled="busy" @click="start" /></div>
    <InlineNotice v-if="error || scan?.isFailed" tone="warning">{{ error ?? t('adminUserProfile.actionFailed') }}</InlineNotice>
    <UProgress v-if="busy" :model-value="scan?.progressPercent ?? 0" />
    <UTable v-if="scan" :data="rows" :columns="columns" class="admin-table"><template #date-cell="{ row }">{{ row.original.lastSeen ? formatDateTime(row.original.lastSeen) : t('common.notAvailable') }}</template><template #empty>{{ busy ? t('common.loading') : t('adminUserProfile.noActiveIp') }}</template></UTable>
  </section>
</template>
