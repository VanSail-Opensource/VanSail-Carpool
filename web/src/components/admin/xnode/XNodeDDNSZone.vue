<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { xnodeApi } from '@/api/xnode'
import { localizedError, useI18n } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

const props = defineProps<{ provider: string }>()
const model = defineModel<string>({ required: true })
const { t } = useI18n()
const zones = shallowRef<{ id: string; name: string }[]>([])
const supported = shallowRef(true)
const loading = shallowRef(false)
const error = shallowRef<string | null>(null)
const latest = createLatestRequest()
let controller: InstanceType<typeof globalThis.AbortController> | undefined
const items = computed(() => {
  const values = zones.value.map(zone => ({ value: zone.name, label: zone.name }))
  if (model.value && !values.some(item => item.value === model.value)) values.unshift({ value: model.value, label: model.value })
  return values
})
async function load() {
  controller?.abort(); controller = new globalThis.AbortController()
  const ticket = latest.begin(); loading.value = !!props.provider; error.value = null; zones.value = []; supported.value = true
  if (!props.provider) return
  try {
    const value = await xnodeApi.providerZones(props.provider, controller.signal)
    if (latest.isCurrent(ticket)) { zones.value = value.items; supported.value = value.supported }
  } catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'nodeForms.zonesFailed') }
  finally { if (latest.isCurrent(ticket)) loading.value = false }
}
watch(() => props.provider, load, { immediate: true })
onScopeDispose(() => { controller?.abort(); latest.dispose() })
</script>

<template>
  <UFormField :label="t('xnode.zone')" :description="t('nodeForms.zoneNameHint')">
    <USelectMenu v-model="model" :items="items" value-key="value" :loading="loading" :disabled="!provider" :search-input="{ placeholder: t('nodeForms.discoverySearch') }" class="w-full" :ui="{ value: 'whitespace-normal break-all', itemLabel: 'whitespace-normal break-all' }">
      <template #empty>{{ t(loading ? 'common.loading' : 'nodeForms.zonesEmpty') }}</template>
    </USelectMenu>
    <p v-if="error" class="text-sm text-error" role="alert">{{ error }}</p>
    <p v-else-if="!loading && !supported" class="text-sm text-muted">{{ t('nodeForms.zonesUnsupported') }}</p>
    <p v-else-if="!loading && provider && !zones.length" class="text-sm text-muted">{{ t('nodeForms.zonesEmpty') }}</p>
    <UButton v-if="provider" variant="link" color="neutral" :loading="loading" :label="t(error ? 'common.tryAgain' : 'common.refresh')" @click="load" />
  </UFormField>
</template>
