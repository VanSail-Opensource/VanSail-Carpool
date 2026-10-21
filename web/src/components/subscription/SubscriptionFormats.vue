<script setup lang="ts">
import { xnodeMemberApi } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import { useSubscriptionData } from './useSubscriptionData'
import SubscriptionFormatDiagnostics from './SubscriptionFormatDiagnostics.vue'
import SubscriptionTuningDiagnostics from './SubscriptionTuningDiagnostics.vue'

const { t } = useI18n()
const { value, loading, error, refresh } = useSubscriptionData(xnodeMemberApi.subscription)
</script>

<template>
  <section class="section-block grid gap-3">
    <UProgress v-if="loading" />
    <p v-if="value?.active && !Object.values(value.formats).some(Boolean)" class="text-sm text-muted">{{ t('memberNodes.noFormats') }}</p>
    <p v-if="value?.incomplete" class="text-xs text-muted">{{ t('memberNodes.incomplete') }}</p>
    <SubscriptionFormatDiagnostics :diagnostics="value?.formatDiagnostics" />
    <SubscriptionTuningDiagnostics :items="value?.connectionOptions" />
    <div v-if="error" class="flex items-center gap-2"><span class="text-sm text-muted">{{ error }}</span><UButton variant="ghost" color="neutral" :label="t('common.tryAgain')" @click="refresh()" /></div>
  </section>
</template>
