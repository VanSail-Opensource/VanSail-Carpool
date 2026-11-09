<script setup lang="ts">
import { computed } from 'vue'
import type { XNodeSettings } from '@/api/xnode'
import SwitchField from '@/components/common/SwitchField.vue'
import { useI18n } from '@/i18n'

const model = defineModel<NonNullable<XNodeSettings['dns']>>({ required: true })
const { t } = useI18n()
const redirect = computed({ get: () => !!model.value.redirect, set: value => { model.value = { ...model.value, redirect: value } } })
</script>

<template>
  <div class="grid gap-4 py-3">
    <p class="text-xs text-muted">{{ t('nodeForms.nativeXrayDnsHint') }}</p>
    <SwitchField id="node-dns-redirect" v-model="redirect" :label="t('xnode.dnsRedirect')" :help="t('xnode.scopedDnsHint')" />
  </div>
</template>
