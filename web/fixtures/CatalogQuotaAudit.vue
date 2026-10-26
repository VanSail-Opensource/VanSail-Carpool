<script setup lang="ts">
import type { Combo } from '@/api/types'
import AdminCatalogEditor from '@/components/admin/AdminCatalogEditor.vue'
const params = new globalThis.URLSearchParams(globalThis.location.search)
const legacy = params.has('legacy-combo')
const stamp = '2026-10-22T00:00:00Z'
const combo: Combo = { id: 'quota-combo', name: 'Member access', description: legacy ? '' : 'Monthly access', price: { currency: 'TXB', minor: legacy ? '0' : '100', display: legacy ? '0.00 TXB' : '1.00 TXB' }, validityDays: 30, trafficLimitBytes: '1073741824', resetStrategy: 'MONTH_ROLLING', active: true, includedSquads: [], ipLookupQuota: 3, whitelistIpQuota: 5, createdAt: stamp, updatedAt: stamp }
function save(payload: Record<string, unknown>) { globalThis.dispatchEvent(new globalThis.CustomEvent('catalog-quota-save', { detail: payload })) }
</script>
<template><AdminCatalogEditor :combo="combo" :squads="[]" :busy="false" @save="save" /></template>
