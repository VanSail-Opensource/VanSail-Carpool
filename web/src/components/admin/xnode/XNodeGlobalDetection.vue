<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeGlobal, XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeFirewallRules from './XNodeFirewallRules.vue'

const model = defineModel<XNodeGlobal>({ required: true })
const { t } = useI18n()
type Policy = NonNullable<XNodeSettings['firewall']>
const policy = computed<Policy>({
  get: () => ({ enabled: true, protect: true, rules: model.value.firewallDetection?.rules ?? {}, ban_seconds: model.value.firewallDetection?.banSeconds ?? 259200 }),
  set: value => { model.value = { ...model.value, firewallDetection: { rules: value.rules ?? {}, banSeconds: value.ban_seconds ?? 259200, migrationNotice: false } } },
})
</script>

<template>
  <section class="grid gap-3">
    <div><h3 class="text-sm font-medium">{{ t('firewall.globalRules') }}</h3><p class="text-xs text-muted">{{ t('firewall.globalRulesHint') }}</p></div>
    <UAlert v-if="model.firewallDetection?.migrationNotice" color="warning" :title="t('firewall.migrationNotice')" />
    <XNodeFirewallRules v-model="policy" rules-only />
  </section>
</template>
