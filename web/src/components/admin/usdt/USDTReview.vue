<script setup lang="ts">
import type { USDTReviewPage } from '@/api/usdt'
import { useI18n } from '@/i18n'
import USDTReviewItem from './USDTReviewItem.vue'
defineProps<{ page: USDTReviewPage; resolving: string | null }>()
const emit = defineEmits<{ refresh: []; more: []; resolve: [id: string, orderId: string, reason: string] }>()
const { t } = useI18n()
</script>

<template>
  <section class="usdt-review">
    <div class="usdt-toolbar"><h3>{{ t('adminUsdt.review') }}</h3><UButton color="neutral" variant="ghost" icon="i-ph-arrows-clockwise" :aria-label="t('common.refresh')" @click="emit('refresh')" /></div>
    <p class="field-hint">{{ t('adminUsdt.reviewHint') }}</p>
    <p v-if="!page.items.length" class="field-hint">{{ t('adminUsdt.noTransfers') }}</p>
    <USDTReviewItem v-for="transfer in page.items" :key="transfer.id" :transfer="transfer" :busy="resolving === transfer.id" @resolve="(id, order, reason) => emit('resolve', id, order, reason)" />
    <UButton v-if="page.nextCursor" color="neutral" variant="outline" :label="t('adminUsdt.more')" @click="emit('more')" />
  </section>
</template>
