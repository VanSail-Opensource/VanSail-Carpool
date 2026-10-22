<script setup lang="ts">
import type { Combo } from '@/api/types'
import AdminCatalogEditor from '@/components/admin/AdminCatalogEditor.vue'
const stamp = '2026-10-22T00:00:00Z'
const combo: Combo = { id: 'quota-combo', name: 'Member access', description: 'Monthly access', price: { currency: 'TXB', minor: '100', display: '1.00 TXB' }, validityDays: 30, trafficLimitBytes: '1073741824', resetStrategy: 'MONTH_ROLLING', active: true, includedSquads: [], rolloverMinRemainingBps: 0, ipLookupQuota: 3, whitelistIpQuota: 5, createdAt: stamp, updatedAt: stamp }
function save(payload: Record<string, unknown>) { globalThis.dispatchEvent(new globalThis.CustomEvent('catalog-quota-save', { detail: payload })) }
</script>
<template><AdminCatalogEditor :combo="combo" :squads="[]" :busy="false" @save="save" /></template>
