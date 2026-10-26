<script setup lang="ts">
import { computed } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import AppShell from '@/components/layout/AppShell.vue'
import ComingSoonLinks from '@/components/dashboard/ComingSoonLinks.vue'
import { useI18n } from '@/i18n'
import AdminCatalogEditor from '@/components/admin/AdminCatalogEditor.vue'
import type { Combo } from '@/api/types'

const { locale } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
const links = new globalThis.URLSearchParams(globalThis.location.search).has('links')
const catalog = new globalThis.URLSearchParams(globalThis.location.search).has('catalog')
const combo: Combo = { id: 'standard', name: 'Standard', description: 'Residential access', price: { currency: 'TXB', minor: '1000', display: '10.00 TXB' }, validityDays: 30, trafficLimitBytes: '107374182400', resetStrategy: 'MONTH_ROLLING', active: true, includedSquads: [], createdAt: '2026-10-10T00:00:00Z', updatedAt: '2026-10-10T00:00:00Z', ipLookupQuota: 2 }
function captureCombo(value: unknown): void { (globalThis.window as unknown as { __savedCombo: unknown }).__savedCombo = value }
</script>

<template><UApp :locale="uiLocale"><AppShell><AdminCatalogEditor v-if="catalog" :combo="combo" :squads="[]" :busy="false" @save="captureCombo" /><ComingSoonLinks v-else-if="links" :has-valid-combo="false" /><RouterView v-else /></AppShell></UApp></template>
