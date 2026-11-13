<script setup lang="ts">
import { computed, watch } from 'vue'
import type { DDNSHostBinding, XNodeEndpoint, XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeDDNSZone from './XNodeDDNSZone.vue'
import XNodeDDNSHosts from './XNodeDDNSHosts.vue'
import { builtinDDNSProvider, ddnsErrors, maxDDNSTTLSeconds, type DDNSJob } from './ddnsValidation'
const job = defineModel<NonNullable<XNodeSettings['ddns']>[number]>({ required: true })
const binding = defineModel<DDNSHostBinding>('binding', { required: true })
defineProps<{ providers: { value: string; label: string }[]; loading: boolean; hosts: readonly XNodeEndpoint[]; used: readonly string[] }>()
defineEmits<{ remove: [] }>()
const { t } = useI18n()
const builtin = computed(() => job.value.provider === builtinDDNSProvider)
const errors = computed(() => ddnsErrors(job.value))
const error = (field: keyof DDNSJob | 'source') => errors.value[field] ? t(`nodeForms.${errors.value[field]}`) : undefined
watch(builtin, value => {
  if (value) job.value = { ...job.value, ipv4: job.value.ipv4 || !job.value.ipv6 ? 'auto' : '', ipv6: job.value.ipv6 ? 'auto' : '' }
})
function enable(family: 'ipv4' | 'ipv6', value: boolean): void { job.value = { ...job.value, [family]: value ? 'auto' : '' } }
watch(() => [job.value.ipv4, job.value.ipv6], () => {
  if (!job.value.ipv4 && binding.value.ipv4HostIds?.length || !job.value.ipv6 && binding.value.ipv6HostIds?.length) binding.value = { ...binding.value, ipv4HostIds: job.value.ipv4 ? binding.value.ipv4HostIds : [], ipv6HostIds: job.value.ipv6 ? binding.value.ipv6HostIds : [] }
})
</script>

<template>
  <div class="ddns-job">
    <div class="ddns-fields">
      <UFormField :label="t('xnode.jobId')" :error="error('id')"><UInput v-model="job.id" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.dnsProvider')" :error="error('provider')">
        <USelectMenu v-model="job.provider" :items="providers" value-key="value" :loading="loading" :search-input="{ placeholder: t('nodeForms.discoverySearch') }" class="w-full">
          <template #empty>{{ t('nodeForms.noProviders') }}</template>
        </USelectMenu>
      </UFormField>
      <p v-if="builtin" class="ddns-hint">{{ t('nodeForms.builtinDDNSHint') }} {{ t('nodeForms.ddnsUpgradeHint') }}</p>
      <template v-else><XNodeDDNSZone v-model="job.zone" :provider="job.provider" :validation-error="error('zone')" /><UFormField :label="t('xnode.recordName')" :error="error('name')" :description="t('nodeForms.ddnsNameHint')"><UInput v-model="job.name" class="w-full" /></UFormField></template>
      <template v-if="builtin"><USwitch :model-value="Boolean(job.ipv4)" :label="t('nodeForms.discoverIPv4')" @update:model-value="enable('ipv4', $event)" /><USwitch :model-value="Boolean(job.ipv6)" :label="t('nodeForms.discoverIPv6')" @update:model-value="enable('ipv6', $event)" /></template>
      <template v-else><UFormField :label="t('xnode.ipv4')" :error="error('ipv4')"><UInput v-model="job.ipv4" class="w-full" /></UFormField><UFormField :label="t('xnode.ipv6')" :error="error('ipv6')"><UInput v-model="job.ipv6" class="w-full" /></UFormField></template>
      <p v-if="error('source')" class="ddns-error" role="alert">{{ error('source') }}</p>
      <XNodeDDNSHosts v-model="binding" :job="job" :hosts="hosts" :used="used" :builtin="builtin" />
      <UFormField :label="t('nodeForms.ipv4Url')" :error="error('ipv4_url')"><UInput v-model="job.ipv4_url" type="url" pattern="https://.*" class="w-full" /></UFormField>
      <UFormField :label="t('nodeForms.ipv6Url')" :error="error('ipv6_url')"><UInput v-model="job.ipv6_url" type="url" pattern="https://.*" class="w-full" /></UFormField>
      <UFormField v-if="!builtin" :label="t('nodeForms.ttlSeconds')" :error="error('ttl_seconds')"><UInput v-model.number="job.ttl_seconds" type="number" min="0" :max="maxDDNSTTLSeconds" step="1" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.interface')" :error="error('interface')"><UInput v-model="job.interface" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.intervalSeconds')" :error="error('interval_seconds')"><UInput v-model.number="job.interval_seconds" type="number" min="0" max="86400" step="1" class="w-full" /></UFormField>
      <p v-if="builtin" class="ddns-hint">{{ t('nodeForms.ddnsSourceHint') }} {{ t('nodeForms.ddnsDistinctHosts') }}</p>
    </div>
    <UButton color="neutral" variant="ghost" icon="i-ph-trash-bold" :aria-label="t('common.delete')" @click="$emit('remove')" />
  </div>
</template>

<style scoped>
.ddns-job { display: flex; gap: 8px; align-items: start; }
.ddns-fields { min-width: 0; flex: 1; display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 12px; }
.ddns-error { grid-column: 1 / -1; margin: 0; color: var(--ui-error); font-size: .75rem; }
.ddns-hint { grid-column: 1 / -1; margin: 0; color: var(--text-muted); font-size: .75rem; }
@media(max-width:600px) { .ddns-fields { grid-template-columns: minmax(0,1fr); } }
</style>
