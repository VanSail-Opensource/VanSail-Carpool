<script setup lang="ts">
import { paygApi } from '@/api/payg'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import { useSubscriptionData } from '@/components/subscription/useSubscriptionData'
import { useI18n } from '@/i18n'
import PAYGSummary from './PAYGSummary.vue'
import PAYGSquadCard from './PAYGSquadCard.vue'

const { t } = useI18n()
const { value: data, loading, error, refresh } = useSubscriptionData(paygApi.get, 15_000, 'payg.loadFailed')
</script>

<template>
  <section class="payg-page">
    <PAYGSummary :data="data" :loading="loading" />
    <div v-if="error" class="payg-page__error">
      <InlineNotice tone="warning">{{ error }}</InlineNotice>
      <UButton color="neutral" variant="outline" :label="t('common.tryAgain')" @click="refresh()" />
    </div>
    <div v-if="loading" class="payg-page__grid" aria-busy="true"><SkeletonBlock v-for="index in 3" :key="index" height="9rem" /></div>
    <template v-else-if="data">
      <p v-if="!data.squads.length" class="payg-page__empty">{{ t('payg.empty') }}</p>
      <div v-else class="payg-page__grid">
        <PAYGSquadCard v-for="(item, index) in data.squads" :key="item.squad.id" :item="item" :style="{ '--entry-delay': `${Math.min(index, 5) * 30}ms` }" />
      </div>
    </template>
  </section>
</template>

<style scoped>
.payg-page { display: grid; gap: .6rem; }
.payg-page__grid { display: grid; gap: .45rem; grid-template-columns: repeat(auto-fit, minmax(min(100%, 20rem), 1fr)); align-items: start; }
.payg-page__empty { margin: 0; padding-block: 1.5rem; text-align: center; color: var(--text-muted); font-size: .8rem; }
.payg-page__error { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: .5rem; }
.payg-page__grid > * { animation: payg-component 220ms ease-out both; animation-delay: calc(var(--catalog-entry-delay, 0ms) + var(--entry-delay, 0ms)); }
@keyframes payg-component { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
@media (prefers-reduced-motion: reduce) { .payg-page__grid > * { animation: none; } }
</style>
