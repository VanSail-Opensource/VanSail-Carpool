<script setup lang="ts">
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'
defineProps<{ state?: string; validUntil?: string }>()
const emit = defineEmits<{ retry: [] }>()
const { t } = useI18n()
</script>
<template>
  <div v-if="state && state !== 'idle'" class="pricing-quote-notice" role="status">
    <template v-if="state === 'ready' && validUntil">{{ t('pricing.quoteValidUntil', { date: formatDateTime(validUntil) }) }}</template>
    <template v-else-if="state !== 'ready'">
      <span>{{ t(`pricing.quoteStates.${state}`) }}</span>
      <UButton v-if="state !== 'calculating'" color="neutral" variant="link" :label="t('common.tryAgain')" @click="emit('retry')" />
    </template>
  </div>
</template>
<style scoped>
.pricing-quote-notice { display:flex; align-items:center; flex-wrap:wrap; gap:.5rem; font-size:.8rem; color:var(--text-muted); line-height:1.5; }
</style>
