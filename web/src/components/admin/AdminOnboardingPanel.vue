<script setup lang="ts">
import { restoreRef } from '@/api/cache/restore'
import { computed, onMounted, onScopeDispose, shallowRef } from 'vue'

import { featuresApi, type OnboardingBundle, type OnboardingLocalizedContent } from '@/api/features'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { localizedError, useI18n } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'
import OnboardingAgreementEditor from './onboarding/OnboardingAgreementEditor.vue'
import OnboardingWelcomeEditor from './onboarding/OnboardingWelcomeEditor.vue'

type BundleKind = 'welcome' | 'agreements'

const { t } = useI18n()
const kind = shallowRef<BundleKind>('welcome')
const bundle = shallowRef<OnboardingBundle | null>(null)
const draft = shallowRef<OnboardingLocalizedContent | null>(null)
const loading = shallowRef(true)
const busy = shallowRef<'save' | 'publish' | null>(null)
const error = shallowRef<string | null>(null)
const message = shallowRef<string | null>(null)
const latest = createLatestRequest()
const kindItems = computed(() => [
  { value: 'welcome', label: t('adminOnboarding.welcome'), disabled: Boolean(busy.value) },
  { value: 'agreements', label: t('adminOnboarding.agreements'), disabled: Boolean(busy.value) },
])

function syncDraft(next: OnboardingBundle): void {
  draft.value = next.draft
}

async function load(nextKind: BundleKind = kind.value): Promise<void> {
  if (busy.value) return
  const ticket = latest.begin()
  if (kind.value !== nextKind) { bundle.value = null; draft.value = null }
  kind.value = nextKind
  loading.value = !restoreRef(`/api/v1/admin/onboarding/content/${nextKind}`, bundle)
  if (!loading.value && bundle.value) syncDraft(bundle.value)
  const originalDraft = JSON.stringify(draft.value)
  error.value = null
  message.value = null
  try {
    const response = await featuresApi.getAdminOnboardingBundle(nextKind)
    if (!latest.isCurrent(ticket) || kind.value !== nextKind || JSON.stringify(draft.value) !== originalDraft) return
    bundle.value = response
    syncDraft(bundle.value)
  } catch (caught) {
    if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'errors.adminLoad')
  } finally {
    if (latest.isCurrent(ticket)) loading.value = false
  }
}

async function save(): Promise<void> {
  if (!bundle.value || !draft.value || busy.value || loading.value) return
  const ticket = latest.begin()
  const originalDraft = JSON.stringify(draft.value)
  busy.value = 'save'
  error.value = null
  message.value = null
  try {
    const response = await featuresApi.saveAdminOnboardingDraft(kind.value, bundle.value.draftRevision, JSON.parse(originalDraft) as OnboardingLocalizedContent)
    if (!latest.isCurrent(ticket)) return
    bundle.value = response
    if (JSON.stringify(draft.value) === originalDraft) syncDraft(response)
    message.value = t('adminOnboarding.saved')
  } catch (caught) {
    if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'errors.adminAction')
  } finally {
    if (latest.isCurrent(ticket)) busy.value = null
  }
}

async function publish(): Promise<void> {
  if (!bundle.value || !draft.value || busy.value || loading.value) return
  const ticket = latest.begin()
  const originalDraft = JSON.stringify(draft.value)
  busy.value = 'publish'
  error.value = null
  message.value = null
  try {
    if (JSON.stringify(bundle.value.draft) !== originalDraft) {
      const response = await featuresApi.saveAdminOnboardingDraft(kind.value, bundle.value.draftRevision, JSON.parse(originalDraft) as OnboardingLocalizedContent)
      if (!latest.isCurrent(ticket)) return
      bundle.value = response
    }
    const response = await featuresApi.publishAdminOnboarding(kind.value, bundle.value.draftRevision)
    if (!latest.isCurrent(ticket)) return
    bundle.value = response
    if (JSON.stringify(draft.value) === originalDraft) syncDraft(response)
    message.value = t('adminOnboarding.published')
  } catch (caught) {
    if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'errors.adminAction')
  } finally {
    if (latest.isCurrent(ticket)) busy.value = null
  }
}

onMounted(() => void load())
onScopeDispose(() => latest.dispose())
</script>

<template>
  <section class="admin-panel onboarding-editor">
    <div class="admin-panel__heading"><div><h2>{{ t('adminOnboarding.title') }}</h2><p>{{ t('adminOnboarding.copy') }}</p></div><div class="row-actions"><UButton color="neutral" variant="outline" icon="i-ph-floppy-disk" :disabled="Boolean(busy) || loading" :loading="busy === 'save'" :label="t('adminOnboarding.saveDraft')" @click="save" /><UButton icon="i-ph-cloud-arrow-up" :disabled="Boolean(busy) || loading" :loading="busy === 'publish'" :label="t('adminOnboarding.publish')" @click="publish" /></div></div>
    <UTabs :model-value="kind" :items="kindItems" variant="link" :content="false" class="admin-workspace-tabs" @update:model-value="load($event as BundleKind)" />
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <InlineNotice v-if="message" tone="success">{{ message }}</InlineNotice>
    <USkeleton v-if="loading" class="h-40" />
    <template v-else-if="bundle && draft">
      <div class="admin-toolbar"><span>{{ kind === 'welcome' ? t('adminOnboarding.welcomeHint') : t('adminOnboarding.agreementHint') }}</span><div class="row-actions"><UBadge color="neutral" variant="subtle" :label="t('adminOnboarding.draftRevision', { revision: bundle.draftRevision })" /><UBadge color="neutral" variant="subtle" :label="t('adminOnboarding.publishedRevision', { revision: bundle.publishedRevision })" /></div></div>
      <OnboardingWelcomeEditor v-if="kind === 'welcome'" :content="draft" @update:content="draft = $event" />
      <OnboardingAgreementEditor v-else :content="draft" @update:content="draft = $event" />
    </template>
  </section>
</template>
