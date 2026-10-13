<script setup lang="ts">
import { computed, shallowRef } from 'vue'

import type { OnboardingLocalizedContent } from '@/api/features'
import { agreementIcon } from '@/components/onboarding/agreementIcons'
import { useI18n } from '@/i18n'
import {
  agreementColors,
  agreementContent,
  agreementIconKeys,
  agreementPairs,
  newAgreementPair,
  normalizeContentID,
  type AgreementColor,
  type BilingualAgreement,
  type EditableAgreement,
} from './onboardingEditor'

const props = defineProps<{ content: OnboardingLocalizedContent }>()
const emit = defineEmits<{ 'update:content': [content: OnboardingLocalizedContent] }>()
const { t } = useI18n()
const pairs = computed(() => agreementPairs(props.content))
const currentIndex = shallowRef<number | null>(null)
const selected = computed(() => currentIndex.value === null ? null : pairs.value[currentIndex.value])
const columns = computed(() => [{ accessorKey: 'id', header: t('adminOnboarding.identifier') }, { id: 'english', header: t('app.english') }, { id: 'chinese', header: t('app.simplifiedChinese') }, { id: 'actions', header: t('adminWorkspace.actions') }])
const canAdd = computed(() => pairs.value.length < 30)

function commit(next: BilingualAgreement[]): void {
  emit('update:content', agreementContent(next))
}

function change(index: number, update: (pair: BilingualAgreement) => BilingualAgreement): void {
  commit(pairs.value.map((pair, position) => position === index ? update(pair) : pair))
}

function updateID(index: number, id: string): void {
  const normalizedID = normalizeContentID(id)
  change(index, (pair) => ({ ...pair, id: normalizedID, english: { ...pair.english, id: normalizedID }, chinese: { ...pair.chinese, id: normalizedID } }))
}

function updateText(index: number, locale: 'english' | 'chinese', field: 'title' | 'body' | 'pageTitle', value: string): void {
  change(index, (pair) => ({ ...pair, [locale]: { ...pair[locale], [field]: value } as EditableAgreement }))
}

function setIcon(index: number, icon: BilingualAgreement['english']['icon']): void {
  change(index, (pair) => ({ ...pair, english: { ...pair.english, icon }, chinese: { ...pair.chinese, icon } }))
}

function setColor(index: number, color: AgreementColor): void {
  change(index, (pair) => ({ ...pair, english: { ...pair.english, color }, chinese: { ...pair.chinese, color } }))
}

function move(index: number, direction: -1 | 1): void {
  const target = index + direction
  if (target < 0 || target >= pairs.value.length) return
  const pageTitles = { english: pairs.value[0].english.pageTitle, chinese: pairs.value[0].chinese.pageTitle }
  const next = [...pairs.value]
  const current = next[index]
  next[index] = next[target]
  next[target] = current
  commit(next.map((pair, position) => ({
    ...pair,
    english: { ...pair.english, ...(position === 0 && pageTitles.english ? { pageTitle: pageTitles.english } : { pageTitle: undefined }) },
    chinese: { ...pair.chinese, ...(position === 0 && pageTitles.chinese ? { pageTitle: pageTitles.chinese } : { pageTitle: undefined }) },
  })))
}

function remove(index: number): void {
  currentIndex.value = null
  if (pairs.value.length <= 1) return
  const pageTitles = { english: pairs.value[0].english.pageTitle, chinese: pairs.value[0].chinese.pageTitle }
  commit(pairs.value.filter((_, position) => position !== index).map((pair, position) => ({
    ...pair,
    english: { ...pair.english, ...(position === 0 && pageTitles.english ? { pageTitle: pageTitles.english } : { pageTitle: undefined }) },
    chinese: { ...pair.chinese, ...(position === 0 && pageTitles.chinese ? { pageTitle: pageTitles.chinese } : { pageTitle: undefined }) },
  })))
}

function add(): void {
  if (canAdd.value) { const index = pairs.value.length; commit([...pairs.value, newAgreementPair()]); currentIndex.value = index }
}
</script>

<template>
  <section class="content-index" :aria-label="t('adminOnboarding.agreements')">
    <UTable class="admin-table" :data="pairs" :columns="columns">
      <template #english-cell="{ row }"><span class="content-excerpt">{{ row.original.english.title }}</span></template>
      <template #chinese-cell="{ row }"><span class="content-excerpt">{{ row.original.chinese.title }}</span></template>
      <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminWorkspace.details')" @click="currentIndex = row.index" /><UButton color="neutral" variant="ghost" icon="i-ph-arrow-up" :disabled="row.index === 0" :aria-label="t('adminOnboarding.moveUp')" @click="move(row.index, -1)" /><UButton color="neutral" variant="ghost" icon="i-ph-arrow-down" :disabled="row.index === pairs.length - 1" :aria-label="t('adminOnboarding.moveDown')" @click="move(row.index, 1)" /><UButton color="error" variant="ghost" icon="i-ph-trash" :disabled="pairs.length <= 1" :aria-label="t('adminOnboarding.removeCard')" @click="remove(row.index)" /></div></template>
    </UTable>
    <UButton color="neutral" variant="outline" icon="i-ph-plus" :disabled="!canAdd" :label="t('adminOnboarding.addAgreement')" @click="add" />
    <USlideover :open="currentIndex !== null" :title="t('adminOnboarding.agreements')" :description="t('adminOnboarding.copy')" :ui="{ content: 'max-w-3xl' }" @update:open="!$event && (currentIndex = null)">
      <template #body>
        <div v-if="selected && currentIndex !== null" class="content-edit-workspace">
          <div class="admin-editor-form">
            <UFormField :label="t('adminOnboarding.identifier')"><UInput :model-value="selected.id" :maxlength="64" @update:model-value="updateID(currentIndex, String($event))" /></UFormField>
            <div class="agreement-card__controls">
              <UFormField :label="t('adminOnboarding.icon')">
                <div class="icon-picker" :aria-label="t('adminOnboarding.icon')">
                  <UButton v-for="icon in agreementIconKeys" :key="icon" color="neutral" variant="outline" square :class="{ 'is-selected': selected.english.icon === icon }" :icon="agreementIcon(icon)" :aria-label="t(`adminOnboarding.icons.${icon}`)" @click="setIcon(currentIndex, icon)" />
                </div>
              </UFormField>
              <UFormField :label="t('adminOnboarding.color')">
                <div class="color-picker" :aria-label="t('adminOnboarding.color')">
                  <UButton v-for="color in agreementColors" :key="color" color="neutral" variant="outline" square :class="['color-picker__item', `color-picker__item--${color}`, { 'is-selected': selected.english.color === color }]" :icon="selected.english.color === color ? 'i-ph-check' : undefined" :aria-label="t(`adminOnboarding.colors.${color}`)" @click="setColor(currentIndex, color)" />
                </div>
              </UFormField>
            </div>
            <div v-if="currentIndex === 0" class="agreement-card__locales">
              <UFormField :label="t('adminOnboarding.englishPageTitle')"><UInput :model-value="selected.english.pageTitle ?? ''" :maxlength="200" @update:model-value="updateText(currentIndex, 'english', 'pageTitle', String($event))" /></UFormField>
              <UFormField :label="t('adminOnboarding.chinesePageTitle')"><UInput :model-value="selected.chinese.pageTitle ?? ''" :maxlength="200" @update:model-value="updateText(currentIndex, 'chinese', 'pageTitle', String($event))" /></UFormField>
            </div>
            <div class="agreement-card__locales">
              <UFormField :label="t('app.english')"><UInput :model-value="selected.english.title" :maxlength="200" @update:model-value="updateText(currentIndex, 'english', 'title', String($event))" /></UFormField>
              <UFormField :label="t('app.simplifiedChinese')"><UInput :model-value="selected.chinese.title" :maxlength="200" @update:model-value="updateText(currentIndex, 'chinese', 'title', String($event))" /></UFormField>
            </div>
            <div class="agreement-card__locales">
              <UFormField :label="t('adminOnboarding.englishBody')"><UTextarea :model-value="selected.english.body" :rows="4" :maxlength="2000" @update:model-value="updateText(currentIndex, 'english', 'body', String($event))" /></UFormField>
              <UFormField :label="t('adminOnboarding.chineseBody')"><UTextarea :model-value="selected.chinese.body" :rows="4" :maxlength="2000" @update:model-value="updateText(currentIndex, 'chinese', 'body', String($event))" /></UFormField>
            </div>
          </div>
          <aside class="content-preview" :aria-label="t('adminWorkspace.overview')"><h4>{{ selected.english.title }}</h4><p>{{ selected.english.body }}</p><h4>{{ selected.chinese.title }}</h4><p>{{ selected.chinese.body }}</p></aside>
        </div>
      </template><template #footer><UButton color="neutral" variant="outline" :label="t('common.close')" @click="currentIndex = null" /></template>
    </USlideover>
  </section>
</template>
<style scoped>
.content-index { display: grid; gap: .75rem; }
.content-excerpt { display: block; max-width: 25ch; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.content-edit-workspace { display: grid; gap: 1.5rem; }
.content-preview { border-top: 1px solid var(--line); padding-top: 1rem; color: var(--text-muted); white-space: pre-wrap; overflow-wrap: anywhere; }
.content-preview h4 { color: var(--text); font-size: 1rem; }
.content-preview p { line-height: 1.65; }
.content-edit-workspace :deep([data-slot='root']) { width: 100%; }
.icon-picker, .color-picker { display: flex; flex-wrap: wrap; gap: .4rem; }
.is-selected { outline: 2px solid var(--accent); }
.agreement-card__controls, .agreement-card__locales, .welcome-card__locales { display: grid; gap: .85rem; }
@media (min-width: 760px) { .agreement-card__locales, .welcome-card__locales { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
