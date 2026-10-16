<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { xnodeMemberApi } from '@/api/xnodeMember'
import { useClipboard } from '@/composables/useClipboard'
import { useI18n } from '@/i18n'
import { useSubscriptionData } from './useSubscriptionData'

const { t } = useI18n()
const { value, loading, error, refresh } = useSubscriptionData(xnodeMemberApi.subscription)
const format = shallowRef('native')
const { copied, copy } = useClipboard()
const copyFailed = shallowRef(false)
const items = computed(() => ['native', 'xray', 'sing-box', 'clash'].map(value => ({ value, label: t(`publicSubscription.formats.${value}`), disabled: !subscription.value?.formats[value] })))
const subscription = value
const link = computed(() => {
  if (!value.value?.active || !value.value.formats[format.value]) return ''
  const url = new globalThis.URL(value.value.downloadUrl || value.value.url, globalThis.location.origin)
  if (!value.value.downloadUrl) {
    const match = /^\/subscription\/([A-Za-z0-9_-]{43})$/.exec(url.pathname)
    if (!match) return ''
    url.pathname = `/api/v1/subscriptions/${match[1]}`
  }
  url.searchParams.set('format', format.value)
  return url.href
})
watch(value, next => {
  if (!next?.formats[format.value]) format.value = items.value.find(item => !item.disabled)?.value ?? 'native'
})
watch(format, () => { copied.value = false; copyFailed.value = false })
async function copyLink() { copyFailed.value = !await copy(link.value) }
</script>

<template>
  <section class="section-block grid gap-3">
    <UFormField :label="t('publicSubscription.format')"><USelect v-model="format" :items="items" :loading="loading" :disabled="!value?.active || !items.some(item => !item.disabled)" class="w-full" /></UFormField>
    <p v-if="value?.active && !items.some(item => !item.disabled)" class="text-sm text-muted">{{ t('memberNodes.noFormats') }}</p>
    <div class="flex flex-wrap gap-2"><UButton icon="i-ph-copy" :label="t(copied ? 'common.copied' : 'publicSubscription.copy')" :disabled="!link" @click="copyLink" /><UButton :to="link || undefined" external target="_blank" rel="noopener noreferrer" color="neutral" variant="outline" :disabled="!link" :label="t('publicSubscription.open')" /></div>
    <p v-if="copyFailed" role="alert" class="text-sm text-muted">{{ t('publicSubscription.copyFailed') }}</p>
    <p v-if="value?.incomplete" class="text-xs text-muted">{{ t('memberNodes.incomplete') }}</p>
    <div v-if="error" class="flex items-center gap-2"><span class="text-sm text-muted">{{ error }}</span><UButton variant="ghost" color="neutral" :label="t('common.tryAgain')" @click="refresh()" /></div>
  </section>
</template>
