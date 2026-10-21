<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeDDNSJob from './XNodeDDNSJob.vue'
import { useDDNSDiscovery } from './useDDNSDiscovery'

const model = defineModel<XNodeSettings>({ required: true })
const { t } = useI18n()
const jobs = computed(() => model.value.ddns ?? [])
const discovery = useDDNSDiscovery()
const providers = computed(() => {
  const values = [...discovery.items.value]
  for (const job of jobs.value) if (job.provider && !values.some(value => value.value === job.provider)) values.push({ value: job.provider, label: job.provider })
  return values
})
function add(): void {
  model.value = { ...model.value, ddns: [...jobs.value, { id: `ddns-${jobs.value.length + 1}`, provider: '', zone: '', name: '', ttl_seconds: 300, interval_seconds: 300 }] }
}
function remove(index: number): void { model.value = { ...model.value, ddns: jobs.value.filter((_, current) => current !== index) } }
</script>

<template>
  <div class="grid gap-4 py-4">
    <div v-if="discovery.error.value" role="alert"><p>{{ discovery.error.value }}</p><UButton :label="t('common.tryAgain')" variant="link" @click="discovery.load" /></div>
    <XNodeDDNSJob v-for="(_, index) in jobs" :key="index" v-model="jobs[index]!" :providers="providers" :loading="discovery.loading.value" @remove="remove(index)" />
    <UButton variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('xnode.addDDNS')" class="justify-self-start" @click="add" />
  </div>
</template>
