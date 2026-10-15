<script setup lang="ts">
import { shallowRef } from 'vue'
import type { PanelAPITokenIssued } from '@/api/adminAPITokens'
import { useI18n } from '@/i18n'
import { copyText } from '@/utils/browserClipboard'

const props = defineProps<{ issued: PanelAPITokenIssued }>()
const emit = defineEmits<{ dismiss: [] }>()
const { t } = useI18n()
const copied = shallowRef(false)
const failed = shallowRef(false)
async function copy(): Promise<void> { copied.value = await copyText(props.issued.token); failed.value = !copied.value }
</script>

<template>
  <section class="grid gap-3 min-w-0" role="status">
    <h3 class="font-medium">{{ t('apiTokens.oneTimeTitle') }}</h3>
    <p class="text-sm text-muted">{{ t('apiTokens.oneTimeHint') }}</p>
    <UInput :model-value="issued.token" :aria-label="t('apiTokens.secret')" readonly class="w-full" :ui="{ base: 'font-mono' }" autocomplete="off" />
    <p v-if="failed" class="text-sm text-error">{{ t('apiTokens.copyFailed') }}</p>
    <div class="flex gap-2"><UButton :label="t(copied ? 'apiTokens.copied' : 'apiTokens.copy')" icon="i-ph-copy" @click="copy" /><UButton color="neutral" variant="ghost" :label="t('apiTokens.dismiss')" @click="emit('dismiss')" /></div>
  </section>
</template>
