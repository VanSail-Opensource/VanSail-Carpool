<script setup lang="ts">
import LoadingPlaceholder from '@/components/common/LoadingPlaceholder.vue'
import { computed } from 'vue'
import { t } from '@/i18n'
import ConnectionLeaseForm from './ConnectionLeaseForm.vue'
import ConnectionObservationList from './ConnectionObservationList.vue'
import ConnectionSummary from './ConnectionSummary.vue'
import { useMemberConnections } from './useMemberConnections'
const state = useMemberConnections()
const data = computed(() => state.data.value)
</script>

<template>
  <section class="page connections-page">
    <header class="connections-page__heading">
      <div><h1>{{ t('connections.title') }}</h1><p>{{ t('connections.subtitle') }}</p></div>
      <UTooltip :text="t('common.refresh')"><UButton icon="i-ph-arrow-clockwise" color="neutral" variant="outline" size="lg" :aria-label="t('common.refresh')" :loading="state.loading.value" @click="state.load" /></UTooltip>
    </header>
    <UAlert v-if="state.error.value" color="warning" icon="i-ph-warning-circle" :title="state.error.value">
      <template #actions><UButton color="neutral" variant="outline" :label="t('common.tryAgain')" :loading="state.loading.value" @click="state.load" /></template>
    </UAlert>
    <ConnectionSummary v-if="data" :data="data" :filtered="!!state.search.value.trim()" />
    <div v-else-if="state.loading.value" class="connections-page__skeleton" :aria-label="t('common.loading')" aria-busy="true"><USkeleton class="h-20 w-full" /><USkeleton class="h-48 w-full" /></div>
    <div v-if="data" class="connections-page__workspace">
      <section class="connections-page__inventory" :aria-busy="state.loading.value">
        <div class="connections-page__toolbar">
          <h2><UIcon name="i-ph-list-bullets" aria-hidden="true" />{{ t('connections.addresses') }}</h2>
          <UInput v-model="state.search.value" icon="i-ph-magnifying-glass" class="connections-page__search" type="search" :aria-label="t('connections.search')" :placeholder="t('connections.search')" />
        </div>
        <LoadingPlaceholder v-if="state.loading.value" variant="compact" class="p-4" />
        <UAlert v-if="data.status === 'stale' || data.status === 'unavailable'" class="connections-page__notice" color="warning" icon="i-ph-cloud-warning" :title="t(`connections.status.${data.status}`)" />
        <p v-if="data.incomplete" role="status" class="connections-page__incomplete"><UIcon name="i-ph-info" aria-hidden="true" />{{ t('connections.incomplete') }}</p>
        <ConnectionObservationList v-if="data.items.length" :items="data.items" :leases="data.settings.enabled" />
        <div v-else class="connections-page__empty" role="status"><UIcon name="i-ph-plugs" aria-hidden="true" /><p>{{ t('connections.status.empty') }}</p></div>
        <footer v-if="data.total > 0" class="connections-page__pagination">
          <p>{{ t('connections.page', { page: data.page, pages: Math.max(1, Math.ceil(data.total / data.pageSize)), total: data.total }) }}</p>
          <UPagination v-model:page="state.page.value" :total="data.total" :items-per-page="data.pageSize" :sibling-count="1" show-edges :disabled="state.loading.value" size="sm" />
        </footer>
      </section>
      <aside class="connections-page__grant"><ConnectionLeaseForm :enabled="data.canGrant" :busy="state.saving.value" :error="state.grantError.value" @grant="state.grant" /></aside>
    </div>
  </section>
</template>

<style scoped>
.connections-page { display: grid; align-content: start; gap: 1.15rem; max-width: 1140px; }
.connections-page__heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
.connections-page__heading h1 { margin: 0; }
.connections-page__heading p { margin: .45rem 0 0; color: var(--text-muted); font-size: .8rem; line-height: 1.5; }
.connections-page__heading > div { min-width: 0; }
.connections-page__workspace, .connections-page__skeleton { display: grid; gap: 1rem; min-width: 0; }
.connections-page__inventory { position: relative; min-width: 0; overflow: hidden; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface); }
.connections-page__toolbar { display: grid; align-items: center; gap: .8rem; padding: 1rem; border-bottom: 1px solid var(--line); }
.connections-page__toolbar h2 { display: flex; align-items: center; gap: .45rem; margin: 0; font-size: .9rem; font-weight: 600; }
.connections-page__toolbar h2 > span { color: var(--text-muted); }
.connections-page__search { min-width: 0; width: 100%; }
.connections-page__notice { margin: .75rem; width: auto; }
.connections-page__incomplete { display: flex; align-items: start; gap: .4rem; margin: .85rem 1rem; font-size: .75rem; color: var(--text-muted); }
.connections-page__incomplete > span { flex: 0 0 auto; margin-top: .1rem; }
.connections-page__empty { display: grid; justify-items: center; gap: .8rem; padding: 3rem 1.5rem; text-align: center; color: var(--text-muted); }
.connections-page__empty > span { font-size: 2rem; color: var(--text-faint); }
.connections-page__empty p { margin: 0; max-width: 30ch; font-size: .85rem; }
.connections-page__pagination { display: grid; justify-items: center; gap: .6rem; padding: .9rem .5rem; border-top: 1px solid var(--line); }
.connections-page__pagination p { margin: 0; font-size: .7rem; color: var(--text-muted); }
.connections-page__grant { min-width: 0; order: -1; }
@media (min-width: 640px) { .connections-page__toolbar { grid-template-columns: 1fr minmax(12rem, .8fr); } }
@media (min-width: 1100px) {
  .connections-page__workspace { grid-template-columns: minmax(0, 1fr) 17rem; align-items: start; }
  .connections-page__grant { position: sticky; top: 1rem; order: 1; }
}
</style>
