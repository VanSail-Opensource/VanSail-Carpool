<script setup lang="ts">
import { computed } from 'vue'
import { getLocale, useI18n } from '@/i18n'
import { readCertificateStatus } from './certificateStatus'
const props = defineProps<{ value?: unknown; fresh: boolean }>()
const { t } = useI18n()
const status = computed(() => readCertificateStatus(props.value))
const expired = computed(() => !!status.value?.expires && Date.parse(status.value.expires) <= Date.now())
function formatDateTime(value: string): string { return new Intl.DateTimeFormat(getLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
const rows = computed(() => status.value ? [
  { label: t('xnode.domain'), value: status.value.domain }, { label: t('certificates.sans'), value: status.value.sans.join(', ') },
  { label: t('certificates.issuer'), value: status.value.issuer }, { label: t('certificates.notBefore'), value: status.value.notBefore ? formatDateTime(status.value.notBefore) : '' },
  { label: t('certificates.expires'), value: status.value.expires ? formatDateTime(status.value.expires) : '' },
  { label: t('certificates.servingFingerprint'), value: status.value.fingerprint },
  ...(status.value.acquiredFingerprint && status.value.acquiredFingerprint !== status.value.fingerprint ? [{ label: t('certificates.acquiredFingerprint'), value: status.value.acquiredFingerprint }] : []),
] : [])
</script>
<template>
  <div class="grid min-w-0 gap-2" data-certificate-status>
    <p class="flex flex-wrap gap-x-3 gap-y-1 text-xs"><span>{{ t(status?.phase ? `certificates.phases.${status.phase}` : 'certificates.noStatus') }}</span><span v-if="status">{{ t(status.active === null ? 'certificates.servingUnknown' : status.active ? 'certificates.serving' : 'certificates.notServing') }}</span><span v-if="expired || status?.expiring" class="text-warning">{{ t(expired ? 'certificates.expired' : 'certificates.expiring') }}</span><span v-if="!fresh" class="text-warning">{{ t('certificates.statusStale') }}</span></p>
    <dl v-if="status" class="grid min-w-0 gap-2 text-xs sm:grid-cols-2"><div v-for="row in rows" :key="row.label" class="min-w-0"><dt class="text-muted">{{ row.label }}</dt><dd class="mt-1 break-all font-mono">{{ row.value || t('common.notAvailable') }}</dd></div></dl>
    <p v-if="status?.errorCode" role="status" class="text-xs text-warning">{{ t('certificates.errorCode') }} <code>{{ status.errorCode }}</code></p>
  </div>
</template>
