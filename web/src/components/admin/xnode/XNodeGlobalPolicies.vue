<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { XNodeGlobal } from '@/api/xnode'
import JsonDocumentField from '@/components/common/JsonDocumentField.vue'
import SwitchField from '@/components/common/SwitchField.vue'
import { useI18n } from '@/i18n'

const model = defineModel<XNodeGlobal>({ required: true })
const emit = defineEmits<{ valid: [value: boolean] }>()
const { t } = useI18n()
const legacy = computed(() => model.value.geodataIntervalHours == null)
const domainsValid = shallowRef(true)
const domains = computed({ get: () => model.value.blockedDomains ?? [], set: (value: unknown) => {
  domainsValid.value = Array.isArray(value) && value.length <= 10_000 && value.every(item => typeof item === 'string' && item.length > 0 && item.length <= 1024)
  if (domainsValid.value) model.value = { ...model.value, blockedDomains: value as string[] }
  emit('valid', domainsValid.value)
} })
const countries = computed({ get: () => (model.value.sourceGeoIp?.selectors ?? ['geoip:cn']).join(', '), set: (value: string) => {
  const selectors = value.toLowerCase().split(/[\s,]+/).filter(Boolean).map(item => item.startsWith('geoip:') ? item : `geoip:${item}`)
  model.value = { ...model.value, sourceGeoIp: { enabled: model.value.sourceGeoIp?.enabled ?? false, selectors } }
} })
const enabled = computed({ get: () => model.value.sourceGeoIp?.enabled ?? false, set: value => { model.value = { ...model.value, sourceGeoIp: { enabled: value, selectors: model.value.sourceGeoIp?.selectors ?? ['geoip:cn'] } } } })
function convertSchedule() { model.value = { ...model.value, geodataIntervalHours: 24, geodataCron: '' } }
</script>

<template>
  <section class="grid gap-4">
    <div v-if="legacy" class="flex flex-wrap items-center gap-3">
      <span class="text-sm text-muted">{{ t('nodeForms.legacySchedule', { value: model.geodataCron || t('nodeAdmin.disabled') }) }}</span>
      <UButton color="neutral" variant="outline" :label="t('nodeForms.convertSchedule')" @click="convertSchedule" />
    </div>
    <UFormField v-else :label="t('nodeForms.refreshHours')" :description="t('nodeForms.refreshHoursHint')"><UInput v-model.number="model.geodataIntervalHours" type="number" min="0" max="8760" step="1" required /></UFormField>
    <SwitchField id="source-geoip-enabled" v-model="enabled" :label="t('nodeForms.sourceGeoIp')" :help="t('nodeForms.sourceGeoIpHint')" />
    <UFormField :label="t('nodeForms.countries')" :description="t('nodeForms.countriesHint')"><UInput v-model="countries" :disabled="!enabled" class="w-full" /></UFormField>
    <JsonDocumentField v-model="domains" :label="t('nodeForms.domainBlocks')" :rows="5" @valid="emit('valid', $event && domainsValid)" />
  </section>
</template>
