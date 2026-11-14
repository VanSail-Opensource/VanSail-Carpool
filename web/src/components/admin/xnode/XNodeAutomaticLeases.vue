<script setup lang="ts">
import { shallowRef } from 'vue'
import type { XNode } from '@/api/xnode'
import { useI18n } from '@/i18n'
import LoadingPlaceholder from '@/components/common/LoadingPlaceholder.vue'
import { formatDateTime } from '@/utils/format'
import { useAutomaticLeases } from './useAutomaticLeases'
import XNodeAutomaticLeaseTable from './XNodeAutomaticLeaseTable.vue'
import XNodeAutomaticLeaseSettings from './XNodeAutomaticLeaseSettings.vue'

defineProps<{ nodes: XNode[] }>()
const { t } = useI18n()
const { data, query, page, loading, error, load } = useAutomaticLeases()
const settingsOpen = shallowRef(false)
</script>

<template>
  <section class="grid min-w-0 gap-5" :aria-label="t('automaticLeases.title')">
    <header class="flex items-start justify-between gap-3"><div><h3 class="font-semibold">{{ t('automaticLeases.title') }}</h3><p class="mt-1 text-sm text-muted">{{ t('automaticLeases.description') }}</p></div><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="loading" @click="load" /></header>
    <dl v-if="data" class="grid grid-cols-2 gap-x-5 gap-y-3 text-sm sm:grid-cols-4">
      <div><dt class="text-muted">{{ t('automaticLeases.active') }}</dt><dd class="mt-1 text-lg font-semibold tabular-nums">{{ data.statistics.active }}</dd></div>
      <div><dt class="text-muted">{{ t('automaticLeases.addresses') }}</dt><dd class="mt-1 tabular-nums">{{ t('automaticLeases.addressCounts', { v4: data.statistics.ipv4, v6: data.statistics.ipv6 }) }}</dd></div>
      <div><dt class="text-muted">{{ t('automaticLeases.createdRenewed') }}</dt><dd class="mt-1 tabular-nums">{{ data.statistics.created }} / {{ data.statistics.renewed }}</dd></div>
      <div><dt class="text-muted">{{ t('automaticLeases.expiredRejected') }}</dt><dd class="mt-1 tabular-nums">{{ data.statistics.expired }} / {{ data.statistics.rejected }}</dd></div>
    </dl>
    <details :open="settingsOpen" @toggle="settingsOpen = ($event.target as HTMLDetailsElement).open"><summary class="cursor-pointer text-sm font-medium">{{ t('automaticLeases.settings') }}</summary><div v-if="settingsOpen" class="mt-4"><XNodeAutomaticLeaseSettings @saved="load" /></div></details>
    <UFormField :label="t('automaticLeases.search')"><UInput v-model="query" :placeholder="t('automaticLeases.searchHint')" icon="i-ph-magnifying-glass" maxlength="128" class="w-full" /></UFormField>
    <p v-if="data" class="text-xs text-muted">{{ t('automaticLeases.updated', { time: formatDateTime(data.fetchedAt) }) }}</p>
    <LoadingPlaceholder v-if="loading" :label="t('adminSection.loadingContent')" :rows="4" />
    <UAlert v-if="error" color="warning" :title="error" :description="data ? t('automaticLeases.stale') : undefined" />
    <UButton v-if="error" class="justify-self-start" variant="ghost" :label="t('common.tryAgain')" @click="load" />
    <p v-if="data && !loading && !data.items.length" role="status" class="py-6 text-sm text-muted">{{ t('automaticLeases.empty') }}</p>
    <XNodeAutomaticLeaseTable v-else-if="data && !loading" :items="data.items" :nodes="nodes" />
    <p v-if="data" class="text-center text-xs text-muted" role="status">{{ t('automaticLeases.pages', { page: data.page, total: Math.max(1, Math.ceil(data.total / data.pageSize)) }) }}</p>
    <UPagination v-if="data && data.total > data.pageSize" v-model:page="page" :items-per-page="data.pageSize" :total="data.total" :sibling-count="1" show-edges :disabled="loading" size="sm" class="justify-self-center" />
    <p class="text-xs text-muted">{{ t('automaticLeases.memoryHint') }}</p>
  </section>
</template>
