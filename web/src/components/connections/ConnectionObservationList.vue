<script setup lang="ts">
import type { MemberConnection } from '@/api/memberConnections'
import { getLocale, t } from '@/i18n'
defineProps<{ items: MemberConnection[]; leases: boolean }>()
function date(value: string | null) { return value ? new Intl.DateTimeFormat(getLocale(), { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value)) : t('connections.notObserved') }
</script>

<template>
  <ul class="divide-y divide-muted" :aria-label="t('connections.title')">
    <li v-for="item in items" :key="item.ip" class="py-4 grid gap-2">
      <span class="font-mono text-sm break-all select-all">{{ item.ip }}</span>
      <dl class="grid grid-cols-1 sm:grid-cols-3 gap-2 text-xs">
        <div v-if="leases" class="flex items-center gap-2"><dt class="inline-flex text-muted" :title="t('connections.initial')"><UIcon name="i-ph-calendar-plus" aria-hidden="true" /><span class="sr-only">{{ t('connections.initial') }}</span></dt><dd>{{ item.initialLeaseAt ? date(item.initialLeaseAt) : t('connections.noLease') }}</dd></div>
        <div class="flex items-center gap-2"><dt class="inline-flex text-muted" :title="t('connections.observed')"><UIcon name="i-ph-eye" aria-hidden="true" /><span class="sr-only">{{ t('connections.observed') }}</span></dt><dd>{{ date(item.lastObservedAt) }}</dd></div>
        <div v-if="leases" class="flex items-center gap-2"><dt class="inline-flex text-muted" :title="t('connections.expiry')"><UIcon name="i-ph-hourglass" aria-hidden="true" /><span class="sr-only">{{ t('connections.expiry') }}</span></dt><dd>{{ item.expiresAt ? date(item.expiresAt) : t('connections.noLease') }}</dd></div>
      </dl>
    </li>
  </ul>
</template>
