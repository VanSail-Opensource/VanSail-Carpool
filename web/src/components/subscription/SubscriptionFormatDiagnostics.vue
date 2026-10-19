<script setup lang="ts">
import { computed } from 'vue'
import type { components } from '@/api/generated'
import { useI18n } from '@/i18n'

const props = defineProps<{ diagnostics?: components['schemas']['XNodeSubscription']['formatDiagnostics'] }>()
const { t } = useI18n()
const messages = computed(() => Object.entries(props.diagnostics ?? {}).flatMap(([format, diagnostic]) => {
  const key = ({ SUBSCRIPTION_FORMAT_RENDER_FAILED: 'renderFailed', SUBSCRIPTION_FORMAT_NO_ENDPOINTS: 'noEndpoints', SUBSCRIPTION_ENDPOINTS_EXCLUDED: 'excluded' } as Record<string, string>)[diagnostic.code]
  return key ? [t(`publicSubscription.diagnostics.${key}`, { format: t(`publicSubscription.formats.${format}`), count: diagnostic.excludedEndpoints })] : []
}))
</script>

<template>
  <ul v-if="messages.length" class="grid gap-1 text-xs text-muted" role="status">
    <li v-for="message in messages" :key="message">{{ message }}</li>
  </ul>
</template>
