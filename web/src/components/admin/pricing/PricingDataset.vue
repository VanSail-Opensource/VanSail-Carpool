<script setup lang="ts">
import { toRef } from 'vue'
import { useI18n } from '@/i18n'
import InlineNotice from '@/components/common/InlineNotice.vue'
import PricingRecordList from './PricingRecordList.vue'
import { usePricingDataset } from './usePricingDataset'
const props = defineProps<{ name: string; title?: string; fields?: string[]; product?: string; user?: string }>()
const { t } = useI18n()
const state = usePricingDataset(props.name, toRef(props, 'product'), toRef(props, 'user'))
</script>
<template>
  <section class="pricing-dataset" :aria-busy="state.loading.value">
    <USkeleton v-if="state.loading.value && !state.items.value.length" class="h-20 w-full" />
    <InlineNotice v-if="state.error.value" tone="warning">{{ state.error.value }}<UButton color="neutral" variant="link" :label="t('common.tryAgain')" @click="state.load()" /></InlineNotice>
    <slot :items="state.items.value"><PricingRecordList v-if="!state.loading.value || state.items.value.length" :title="title ?? t('pricing.calculationTrace')" :items="state.items.value" :fields="fields" /></slot>
    <div v-if="state.nextCursor.value || state.truncated.value" class="row-actions">
      <span>{{ t('pricing.partialRecords', { count: state.items.value.length }) }}</span>
      <UButton v-if="state.nextCursor.value" color="neutral" variant="outline" :loading="state.loading.value" :label="t('pricing.loadMore')" @click="state.load(true)" />
    </div>
  </section>
</template>
<style scoped>.pricing-dataset { display:grid; gap:.75rem; min-width:0; }.pricing-dataset .row-actions { font-size:.8rem; color:var(--text-muted); }</style>
