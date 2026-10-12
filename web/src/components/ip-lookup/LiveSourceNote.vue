<script setup lang="ts">
import type { IPLookupLiveMeta } from '@/api/ipLookup'
import { computed } from 'vue'
import { t } from '@/i18n'
const props = defineProps<{ meta: IPLookupLiveMeta }>()
const time = computed(() => {
  if (!props.meta.observedAt) return ''
  if (/^\d{4}-\d{2}-\d{2}$/.test(props.meta.observedAt)) return new Date(props.meta.observedAt).toLocaleDateString(undefined, { timeZone: 'UTC' })
  const raw = /^\d{4}-\d{2}-\d{2}T[\d:.]+$/.test(props.meta.observedAt) ? `${props.meta.observedAt}Z` : props.meta.observedAt
  const date = new Date(raw)
  return Number.isNaN(date.getTime()) ? props.meta.observedAt : date.toLocaleString()
})
const message = computed(() => props.meta.status === 'error' || props.meta.status === 'restricted' ? t(`ipLookupLive.errors.${props.meta.errorCode || 'IP_DETAILS_UNAVAILABLE'}`) : t(`ipLookupLive.status.${props.meta.status}`))
</script>

<template>
  <div class="live-source-note"><p><UIcon :name="meta.status === 'error' || meta.status === 'restricted' ? 'i-ph-warning-circle' : 'i-ph-info'" aria-hidden="true" /><span>{{ meta.source }}</span><span v-if="meta.status !== 'success'">{{ $t(`ipLookupLive.status.${meta.status}`) }}</span></p><p>{{ message }}<time v-if="time" :datetime="meta.observedAt">{{ time }}</time></p></div>
</template>

<style scoped>
.live-source-note { margin: 0.3rem 0 0; font-size: 0.7rem; color: var(--text-muted); overflow-wrap: anywhere; }
.live-source-note p { display: flex; flex-wrap: wrap; align-items: center; gap: 0.25rem 0.45rem; }
.live-source-note p { margin: 0.3rem 0; }
</style>
