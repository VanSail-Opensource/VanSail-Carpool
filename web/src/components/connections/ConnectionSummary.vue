<script setup lang="ts">
import type { MemberConnectionPage } from '@/api/memberConnections'
import { getLocale, t } from '@/i18n'
import { formatDateTime } from '@/utils/format'
defineProps<{ data: MemberConnectionPage; filtered: boolean }>()
</script>

<template>
  <dl class="connection-summary">
    <div class="connection-summary__item">
      <dt><UIcon name="i-ph-globe" aria-hidden="true" />{{ t(filtered ? 'connections.matching' : 'connections.addresses') }}</dt>
      <dd>{{ new Intl.NumberFormat(getLocale()).format(data.total) }}</dd>
    </div>
    <div class="connection-summary__item">
      <dt><UIcon name="i-ph-shield-check" aria-hidden="true" />{{ t('connections.leaseDuration') }}</dt>
      <dd>{{ data.settings.enabled ? t('connections.hours', { count: data.settings.durationHours }) : t('connections.leasesDisabled') }}</dd>
    </div>
    <div class="connection-summary__item connection-summary__item--updated">
      <dt><UIcon name="i-ph-clock" aria-hidden="true" />{{ t('connections.updated') }}</dt>
      <dd><time :datetime="data.fetchedAt">{{ formatDateTime(data.fetchedAt) }}</time></dd>
    </div>
  </dl>
</template>

<style scoped>
.connection-summary { display: grid; grid-template-columns: 1fr 1fr; margin: 0; padding: .2rem 0; gap: .85rem; }
.connection-summary__item { display: grid; align-content: start; gap: .35rem; min-width: 0; padding: .4rem 0; }
.connection-summary__item dt { display: flex; align-items: center; gap: .4rem; color: var(--text-muted); font-size: .72rem; }
.connection-summary__item dd { margin: 0; font-size: 1.35rem; font-weight: 650; line-height: 1.2; font-variant-numeric: tabular-nums; }
.connection-summary__item--updated { grid-column: 1 / -1; display: flex; align-items: center; gap: .6rem; padding-top: .65rem; border-top: 1px solid var(--line); }
.connection-summary__item--updated dd { font-size: .75rem; font-weight: 500; color: var(--text-muted); }
@media (min-width: 640px) {
  .connection-summary { grid-template-columns: .75fr .75fr 1fr; padding: .35rem 0 .75rem; }
  .connection-summary__item + .connection-summary__item { border-inline-start: 1px solid var(--line); padding-inline-start: 1rem; }
  .connection-summary__item--updated { grid-column: auto; display: grid; border-top: 0; padding-top: .4rem; gap: .35rem; }
  .connection-summary__item--updated dd { font-size: .95rem; line-height: 1.7; color: var(--text); }
}
</style>
