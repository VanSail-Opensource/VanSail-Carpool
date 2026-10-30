<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import StatisticsNodes from '@/components/statistics/StatisticsNodes.vue'
import type { StatisticsNodesSnapshot } from '@/api/types'
import '@/styles/statistics-01.css'
import '@/styles/statistics-02.css'

const params = new globalThis.URLSearchParams(globalThis.location.search)
const loading = shallowRef(params.has('slow'))
if (loading.value) globalThis.setTimeout(() => { loading.value = false }, 1800)
const snapshot = computed<StatisticsNodesSnapshot | null>(() => loading.value ? null : ({
  generatedAt: new Date().toISOString(), stale: params.has('stale-metrics'),
  nodes: params.has('empty') ? [] : [{ uuid: '11111111-1111-4111-8111-111111111111',
    name: 'Amsterdam', countryCode: 'NL', online: !params.has('offline'),
    usersOnline: params.has('zero-metrics') ? 0 : 2, usersOnlineAvailable: !params.has('unknown-metrics'),
    rxBytesPerSec: '2000', txBytesPerSec: '500', xrayVersion: '26.10.10', multiplier: 1,
  }],
}))
</script>

<template><StatisticsNodes :snapshot="snapshot" :loading="loading" /></template>
