<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeEndpointPreview } from '@/api/xnode'
import { useI18n } from '@/i18n'
import { endpointFieldLabel } from './endpointLabels'
const props = defineProps<{ clients: XNodeEndpointPreview['clients'] }>()
const { t } = useI18n()
const clients = computed(() => props.clients.filter(item => item.format !== 'base64'))
const reason = (code: string) => { const key = `endpointOptions.reasons.${code}`, value = t(key); return value === key ? t('endpointOptions.reasons.unsupported') : value }
</script>
<template>
  <section class="grid gap-2 min-w-0">
    <h4 class="text-sm font-medium">{{ t('diy.compatibility') }}</h4>
    <div v-for="client in clients" :key="client.format" class="grid min-w-0 gap-1 border-b border-default py-2">
      <div class="flex flex-wrap items-center justify-between gap-2"><strong class="text-sm">{{ t(`publicSubscription.formats.${client.format}`) }}</strong><UBadge :color="client.supported ? 'success' : 'warning'" variant="subtle">{{ t(client.supported ? 'endpointOptions.supported' : 'endpointOptions.excluded') }}</UBadge></div>
      <p v-for="code in client.reasons" :key="code" class="text-xs text-muted">{{ reason(code) }}</p>
      <p v-if="client.omittedOptions.length" class="text-xs text-muted break-words">{{ t('endpointOptions.omitted', { fields: client.omittedOptions.map(endpointFieldLabel).join(', ') }) }}</p>
    </div>
  </section>
</template>
