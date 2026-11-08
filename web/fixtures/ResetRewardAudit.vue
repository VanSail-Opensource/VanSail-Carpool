<script setup lang="ts">
import { ref } from 'vue'
import type { LuckyDrawAdmin, LuckyDrawWrite } from '@/api/features'
import AdminLuckyDrawEditor from '@/components/admin/activity/AdminLuckyDrawEditor.vue'
import { useI18n } from '@/i18n'
const { t } = useI18n()
const params = new globalThis.URLSearchParams(globalThis.location.search)
const open = ref(true)
const stamp = new Date().toISOString()
const draw: LuckyDrawAdmin | null = params.has('empty') ? null : {
  id: 'audit-draw', name: 'Audit draw', description: '', kind: 'instant', status: 'open', enabled: true,
  feeTxbMinor: '100', expectedParticipation: 100, threshold: 0, seats: 0,
  prizes: [{ id: 'prize-a', name: 'Audit prize', probabilityBps: 10000, reward: { kind: params.has('legacy') ? 'traffic_reset' : 'none' } }],
  createdAt: stamp, updatedAt: stamp,
}
function save(value: LuckyDrawWrite): void { Object.assign(globalThis, { resetRewardAuditSaved: value }) }
</script>
<template><UApp><main class="browser-console"><AdminLuckyDrawEditor v-if="open" :draw="draw" :busy="params.has('busy')" :error="params.has('save-error') ? t('adminDrawRedesign.invalid') : null" :combo-items="[]" :squad-items="[]" @save="save" @cancel="open = false" /></main></UApp></template>
