<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'

const model = defineModel<XNodeSettings>({ required: true })
const { t } = useI18n()
const jobs = computed(() => model.value.ddns ?? [])
function add(): void {
  model.value = { ...model.value, ddns: [...jobs.value, { id: `ddns-${jobs.value.length + 1}`, provider: '', zone: '', name: '', ttl_seconds: 300, interval_seconds: 300 }] }
}
function remove(index: number): void { model.value = { ...model.value, ddns: jobs.value.filter((_, current) => current !== index) } }
</script>

<template>
  <div class="grid gap-4 py-4">
    <div v-for="(job, index) in jobs" :key="index" class="ddns-job">
      <div class="ddns-fields">
        <UFormField :label="t('xnode.jobId')"><UInput v-model="job.id" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.dnsProvider')"><UInput v-model="job.provider" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.zone')"><UInput v-model="job.zone" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.recordName')"><UInput v-model="job.name" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.ipv4')"><UInput v-model="job.ipv4" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.ipv6')"><UInput v-model="job.ipv6" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.interface')"><UInput v-model="job.interface" class="w-full" /></UFormField>
        <UFormField :label="t('xnode.intervalSeconds')"><UInput v-model.number="job.interval_seconds" type="number" min="1" max="86400" class="w-full" /></UFormField>
      </div>
      <UButton color="neutral" variant="ghost" icon="i-ph-trash-bold" :aria-label="t('common.delete')" @click="remove(index)" />
    </div>
    <UButton variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('xnode.addDDNS')" class="justify-self-start" @click="add" />
  </div>
</template>

<style scoped>
.ddns-job { display: flex; gap: 8px; align-items: start; }
.ddns-fields { flex: 1; display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 12px; }
@media(max-width:600px) { .ddns-fields { grid-template-columns: minmax(0,1fr); } }
</style>
