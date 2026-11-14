<script setup lang="ts">
import type { DIYField, DIYOverrides } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import DIYFieldControl from './DIYFieldControl.vue'
const model = defineModel<DIYOverrides>({ required: true })
defineProps<{ fields: DIYField[]; disabled?: boolean; errors: Record<string, string> }>()
const { t } = useI18n()
const icons: Record<string, string> = { tls: 'i-ph-shield-check', shadowsocks: 'i-ph-paper-plane-tilt', xhttp: 'i-ph-arrows-left-right', xmux: 'i-ph-intersect', mux: 'i-ph-git-branch', singboxMux: 'i-ph-git-branch', mihomoMux: 'i-ph-git-branch', sockopt: 'i-ph-plugs-connected', download: 'i-ph-download-simple' }
function set(key: string, value: unknown) {
  const next = { ...model.value } as Record<string, unknown>
  if (value === undefined) delete next[key]
  else next[key] = value
  model.value = next as DIYOverrides
}
const value = (key: string) => (model.value as Record<string, unknown>)[key]
</script>
<template>
  <section class="grid gap-3 min-w-0">
    <div class="grid gap-x-5 gap-y-4 sm:grid-cols-2">
      <DIYFieldControl v-for="field in fields" :key="field.key" :field="field" :model-value="value(field.key)" :disabled="disabled" :error="errors[field.key] ? t(`diy.errors.${errors[field.key]}`) : undefined" :class="field.kind === 'headers' ? 'sm:col-span-2' : ''" :icon="icons[field.group]" compact @update:model-value="set(field.key, $event)" />
    </div>
  </section>
</template>
