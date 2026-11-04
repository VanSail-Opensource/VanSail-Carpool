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
        <div v-if="leases"><dt class="text-muted">{{ t('connections.initial') }}</dt><dd>{{ item.initialLeaseAt ? date(item.initialLeaseAt) : t('connections.noLease') }}</dd></div>
        <div><dt class="text-muted">{{ t('connections.observed') }}</dt><dd>{{ date(item.lastObservedAt) }}</dd></div>
        <div v-if="leases"><dt class="text-muted">{{ t('connections.expiry') }}</dt><dd>{{ item.expiresAt ? date(item.expiresAt) : t('connections.noLease') }}</dd></div>
      </dl>
    </li>
  </ul>
</template>
