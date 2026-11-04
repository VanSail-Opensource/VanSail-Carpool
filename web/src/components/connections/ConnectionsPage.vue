<script setup lang="ts">
import { computed } from 'vue'
import { t } from '@/i18n'
import ConnectionLeaseForm from './ConnectionLeaseForm.vue'
import ConnectionObservationList from './ConnectionObservationList.vue'
import { useMemberConnections } from './useMemberConnections'
const state = useMemberConnections()
const data = computed(() => state.data.value)
</script>

<template>
  <section class="page grid gap-5 max-w-3xl mx-auto">
    <header class="flex items-center justify-between gap-3">
      <h1 class="text-xl">{{ t('connections.title') }}</h1>
      <UButton icon="i-ph-arrow-clockwise" color="neutral" variant="ghost" :aria-label="t('common.refresh')" :loading="state.loading.value" @click="state.load" />
    </header>
    <ConnectionLeaseForm :enabled="!!data?.canGrant" :busy="state.saving.value" :error="state.grantError.value" @grant="state.grant" />
    <UFormField :label="t('connections.search')"><UInput v-model="state.search.value" class="w-full" type="search" /></UFormField>
    <UProgress v-if="state.loading.value" :aria-label="t('common.loading')" />
    <UAlert v-if="state.error.value" color="warning" :title="state.error.value" />
    <template v-if="data">
      <UAlert v-if="data.status !== 'available'" :color="data.status === 'empty' ? 'neutral' : 'warning'" :title="t(`connections.status.${data.status}`)" />
      <p v-if="data.incomplete" role="status" class="text-sm text-muted">{{ t('connections.incomplete') }}</p>
      <ConnectionObservationList :items="data.items" :leases="data.settings.enabled" />
      <footer v-if="data.total > 0" class="grid gap-2 justify-items-center">
        <p class="text-xs text-muted">{{ t('connections.page', { page: data.page, pages: Math.max(1, Math.ceil(data.total / data.pageSize)), total: data.total }) }}</p>
        <UPagination v-model:page="state.page.value" :total="data.total" :items-per-page="data.pageSize" :sibling-count="1" show-edges :disabled="state.loading.value" size="sm" />
      </footer>
    </template>
  </section>
</template>
