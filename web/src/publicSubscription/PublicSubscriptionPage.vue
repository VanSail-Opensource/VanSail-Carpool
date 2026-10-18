<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import { useI18n } from '@/i18n'
import { formatBytes, formatDate } from '@/utils/format'
import { createLatestRequest } from '@/utils/latestRequest'
import { usePublicSubscription, type SubscriptionFormat } from './usePublicSubscription'

const { t, locale, locales, setLocale } = useI18n()
const props = defineProps<{ token?: string }>()
const token = props.token ?? globalThis.location.pathname.split('/')[2] ?? ''
const subscription = usePublicSubscription(token)
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
const languages = computed(() => locales.value.map(value => ({ value, label: t(value === 'en' ? 'app.english' : 'app.simplifiedChinese') })))
const formats = computed(() => (['base64', 'native', 'xray', 'sing-box', 'clash', 'stash'] as SubscriptionFormat[]).map(value => ({ value, label: t(`publicSubscription.formats.${value}`), disabled: !subscription.metadata.value?.formats[value] })))
const qrOpen = shallowRef(false)
const qr = shallowRef<string | null>(null)
const qrFailed = shallowRef(false)
const latest = createLatestRequest()
watch(() => [qrOpen.value, subscription.link.value] as const, async ([open, link]) => {
  const generation = latest.begin()
  qr.value = null; qrFailed.value = false
  if (!open || !link) { qrOpen.value = false; return }
  try {
    const { default: QRCode } = await import('qrcode')
    const value = await QRCode.toDataURL(link, { width: 280, margin: 3, errorCorrectionLevel: 'M' })
    if (latest.isCurrent(generation)) qr.value = value
  } catch { if (latest.isCurrent(generation)) qrFailed.value = true }
})
onScopeDispose(() => latest.dispose())
</script>

<template>
  <UApp :locale="uiLocale">
    <main class="subscription-page">
      <header class="subscription-header"><strong>{{ t('app.name') }}</strong><USelect :model-value="locale" :items="languages" :aria-label="t('app.language')" @update:model-value="setLocale" /></header>
      <section class="subscription-content">
        <h1>{{ t('publicSubscription.title') }}</h1>
        <div v-if="subscription.loading.value" class="grid gap-4" role="status" :aria-label="t('common.loading')"><USkeleton class="h-16 w-full" /><USkeleton class="h-12 w-full" /><UProgress /></div>
        <UAlert v-else-if="subscription.failed.value" color="warning" variant="soft" :title="t('publicSubscription.unavailable')" :description="t('publicSubscription.unavailableHint')" :actions="[{ label: t('common.tryAgain'), onClick: subscription.load }]" />
        <div v-else-if="subscription.metadata.value" class="grid gap-5 min-w-0">
          <div class="subscription-facts"><div><small>{{ t('publicSubscription.account') }}</small><strong>{{ subscription.metadata.value.username }}</strong></div><div><small>{{ t('publicSubscription.usage') }}</small><strong>{{ formatBytes(subscription.metadata.value.usedBytes) }} / {{ subscription.metadata.value.limitBytes === '0' ? t('publicSubscription.unlimited') : formatBytes(subscription.metadata.value.limitBytes) }}</strong></div><div><small>{{ t('publicSubscription.expiry') }}</small><strong>{{ formatDate(subscription.metadata.value.expiresAt ?? undefined) }}</strong></div></div>
          <UAlert v-if="subscription.metadata.value.incomplete" color="warning" variant="soft" :title="t('publicSubscription.incomplete')" />
          <UAlert v-if="!subscription.metadata.value.active" color="neutral" variant="soft" :title="t('publicSubscription.noAccess')" />
          <UAlert v-else-if="!Object.values(subscription.metadata.value.formats).some(Boolean)" color="neutral" variant="soft" :title="t('memberNodes.noFormats')" />
          <UFormField :label="t('publicSubscription.format')"><USelect v-model="subscription.format.value" :items="formats" :disabled="!subscription.metadata.value.active || !formats.some(item => !item.disabled)" class="w-full" /></UFormField>
          <UInput :model-value="subscription.link.value" readonly :aria-label="t('publicSubscription.url')" class="w-full font-mono" />
          <div class="subscription-actions"><UButton icon="i-ph-copy" :label="t(subscription.copied.value ? 'common.copied' : 'publicSubscription.copy')" :disabled="!subscription.link.value" @click="subscription.copy" /><UButton color="neutral" variant="outline" icon="i-ph-qr-code" :label="t('publicSubscription.qr')" :disabled="!subscription.link.value" @click="qrOpen = true" /><UButton v-if="subscription.importLink.value" :to="subscription.importLink.value" external color="neutral" variant="outline" :label="t('publicSubscription.import')" /><UButton :to="subscription.link.value || undefined" external target="_blank" rel="noopener noreferrer" color="neutral" variant="ghost" :disabled="!subscription.link.value" :label="t('publicSubscription.open')" /></div>
          <p v-if="subscription.copyFailed.value" role="alert" class="text-sm text-muted">{{ t('publicSubscription.copyFailed') }}</p>
        </div>
      </section>
      <UModal v-model:open="qrOpen" :title="t('publicSubscription.qr')" :description="t('publicSubscription.qrHint')"><template #body><div class="grid place-items-center min-h-72"><img v-if="qr" :src="qr" :alt="t('publicSubscription.qr')" width="280" height="280" class="max-w-full" /><UAlert v-else-if="qrFailed" color="warning" :title="t('publicSubscription.qrFailed')" /><USkeleton v-else class="h-72 w-72 max-w-full" /></div></template></UModal>
    </main>
  </UApp>
</template>

<style scoped>
.subscription-page { max-width: 720px; padding: 24px; margin: 0 auto; }
.subscription-header { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.subscription-content { display: grid; grid-template-columns: minmax(0,1fr); gap: 24px; padding: 36px 0; }
h1 { margin: 0; font-size: 24px; }
.subscription-facts { display: grid; gap: 18px; }
.subscription-facts div { display: grid; gap: 4px; min-width: 0; }
.subscription-facts strong { overflow-wrap: anywhere; font-weight: 500; }
small { color: var(--text-muted); }
.subscription-actions { display: flex; flex-wrap: wrap; gap: 8px; }
@media(min-width:640px) { .subscription-facts { grid-template-columns: repeat(3,minmax(0,1fr)); } }
@media(max-width:639px) { .subscription-page { padding: 20px 16px; } .subscription-content { padding: 28px 0; } }
</style>
