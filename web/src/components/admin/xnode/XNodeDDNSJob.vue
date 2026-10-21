<script setup lang="ts">
import type { XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeDDNSZone from './XNodeDDNSZone.vue'
const job = defineModel<NonNullable<XNodeSettings['ddns']>[number]>({ required: true })
defineProps<{ providers: { value: string; label: string }[]; loading: boolean }>()
defineEmits<{ remove: [] }>()
const { t } = useI18n()
</script>

<template>
  <div class="ddns-job">
    <div class="ddns-fields">
      <UFormField :label="t('xnode.jobId')"><UInput v-model="job.id" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.dnsProvider')">
        <USelectMenu v-model="job.provider" :items="providers" value-key="value" :loading="loading" :search-input="{ placeholder: t('nodeForms.discoverySearch') }" class="w-full">
          <template #empty>{{ t('nodeForms.noProviders') }}</template>
        </USelectMenu>
      </UFormField>
      <XNodeDDNSZone v-model="job.zone" :provider="job.provider" />
      <UFormField :label="t('xnode.recordName')"><UInput v-model="job.name" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.ipv4')"><UInput v-model="job.ipv4" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.ipv6')"><UInput v-model="job.ipv6" class="w-full" /></UFormField>
      <UFormField :label="t('nodeForms.ipv4Url')"><UInput v-model="job.ipv4_url" type="url" pattern="https://.*" class="w-full" /></UFormField>
      <UFormField :label="t('nodeForms.ipv6Url')"><UInput v-model="job.ipv6_url" type="url" pattern="https://.*" class="w-full" /></UFormField>
      <UFormField :label="t('nodeForms.ttlSeconds')"><UInput v-model.number="job.ttl_seconds" type="number" min="0" step="1" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.interface')"><UInput v-model="job.interface" class="w-full" /></UFormField>
      <UFormField :label="t('xnode.intervalSeconds')"><UInput v-model.number="job.interval_seconds" type="number" min="0" max="86400" step="1" class="w-full" /></UFormField>
    </div>
    <UButton color="neutral" variant="ghost" icon="i-ph-trash-bold" :aria-label="t('common.delete')" @click="$emit('remove')" />
  </div>
</template>

<style scoped>
.ddns-job { display: flex; gap: 8px; align-items: start; }
.ddns-fields { min-width: 0; flex: 1; display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 12px; }
@media(max-width:600px) { .ddns-fields { grid-template-columns: minmax(0,1fr); } }
</style>
