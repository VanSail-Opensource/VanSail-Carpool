<script setup lang="ts">
import type { MemberConnection } from '@/api/memberConnections'
import { getLocale, t } from '@/i18n'
defineProps<{ items: MemberConnection[]; leases: boolean }>()
function date(value: string | null) { return value ? new Intl.DateTimeFormat(getLocale(), { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(value)) : t('connections.notObserved') }
</script>

<template>
  <ul class="connection-list" :aria-label="t('connections.title')">
    <li v-for="item in items" :key="item.ip" class="connection-list__row">
      <div class="connection-list__identity"><span class="connection-list__icon"><UIcon name="i-ph-globe-hemisphere-west" aria-hidden="true" /></span><span class="connection-list__address">{{ item.ip }}</span><span class="connection-list__family">{{ t(item.ip.includes(':') ? 'connections.ipv6' : 'connections.ipv4') }}</span></div>
      <dl class="connection-list__details" :class="{ 'connection-list__details--leases': leases }">
        <div><dt><UIcon name="i-ph-eye" aria-hidden="true" />{{ t('connections.observed') }}</dt><dd><time v-if="item.lastObservedAt" :datetime="item.lastObservedAt">{{ date(item.lastObservedAt) }}</time><span v-else>{{ t('connections.notObserved') }}</span></dd></div>
        <div v-if="leases"><dt><UIcon name="i-ph-calendar-plus" aria-hidden="true" />{{ t('connections.initial') }}</dt><dd><time v-if="item.initialLeaseAt" :datetime="item.initialLeaseAt">{{ date(item.initialLeaseAt) }}</time><span v-else>{{ t('connections.noLease') }}</span></dd></div>
        <div v-if="leases"><dt><UIcon name="i-ph-hourglass" aria-hidden="true" />{{ t('connections.expiry') }}</dt><dd><time v-if="item.expiresAt" :datetime="item.expiresAt">{{ date(item.expiresAt) }}</time><span v-else>{{ t('connections.noLease') }}</span></dd></div>
      </dl>
    </li>
  </ul>
</template>

<style scoped>
.connection-list { list-style: none; margin: 0; padding: 0; }
.connection-list__row { display: grid; gap: .85rem; padding: 1rem; border-bottom: 1px solid var(--line); transition: background-color 160ms ease; }
.connection-list__row:last-child { border-bottom: 0; }
.connection-list__row:hover { background: var(--surface-raised); }
.connection-list__identity { display: flex; align-items: center; gap: .55rem; min-width: 0; }
.connection-list__icon { display: inline-flex; align-items: center; justify-content: center; flex: 0 0 auto; width: 1.85rem; height: 1.85rem; color: var(--text-muted); background: var(--surface-raised); border-radius: .6rem; }
.connection-list__address { min-width: 0; font-family: var(--font-mono); font-size: .82rem; font-weight: 600; overflow-wrap: anywhere; user-select: all; }
.connection-list__family { margin-inline-start: auto; flex: 0 0 auto; color: var(--text-faint); font-family: var(--font-mono); font-size: .65rem; }
.connection-list__details { display: grid; gap: .7rem; margin: 0; }
.connection-list__details--leases { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.connection-list__details--leases > div:first-child { grid-column: 1 / -1; }
.connection-list__details dt { display: flex; align-items: center; gap: .3rem; color: var(--text-muted); font-size: .64rem; line-height: 1.4; }
.connection-list__details dd { margin: .2rem 0 0; font-size: .72rem; font-variant-numeric: tabular-nums; color: var(--text); }
@media (min-width: 480px) { .connection-list__details--leases { grid-template-columns: repeat(3, minmax(0, 1fr)); } .connection-list__details--leases > div:first-child { grid-column: auto; } }
@media (prefers-reduced-motion: reduce) { .connection-list__row { transition: none; } }
</style>
