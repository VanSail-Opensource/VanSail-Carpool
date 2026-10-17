<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeSettings } from '@/api/xnode'
import SwitchField from '@/components/common/SwitchField.vue'
import { useI18n } from '@/i18n'

const model = defineModel<NonNullable<XNodeSettings['dns']>>({ required: true })
const { t } = useI18n()
const modes = computed(() => [{ value: 'xray', label: t('nodeForms.xrayResolver') }, ...(model.value.mode === 'smartdns' ? [{ value: 'smartdns', label: t('nodeForms.legacyResolver') }] : [])])
const mode = computed({ get: () => model.value.mode || 'xray', set: value => { model.value = { ...model.value, mode: value } } })
const redirect = computed({ get: () => !!model.value.redirect, set: value => { model.value = { ...model.value, redirect: value } } })
const upstreams = computed({ get: () => model.value.upstreams?.join('\n') ?? '', set: value => { model.value = { ...model.value, upstreams: value.split(/\s+/).filter(Boolean) } } })
</script>

<template>
  <div class="grid gap-4 py-3">
    <UFormField :label="t('nodeForms.resolver')"><USelect v-model="mode" :items="modes" class="w-full" /></UFormField>
    <UFormField :label="t('nodeForms.upstreams')" :description="t('nodeForms.upstreamsHint')"><UTextarea v-model="upstreams" :rows="4" class="w-full" /></UFormField>
    <SwitchField id="node-dns-redirect" v-model="redirect" :label="t('xnode.dnsRedirect')" :help="t('xnode.scopedDnsHint')" />
  </div>
</template>
