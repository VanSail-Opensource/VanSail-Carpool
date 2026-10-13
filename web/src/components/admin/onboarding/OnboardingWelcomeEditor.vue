<script setup lang="ts">
import { computed, shallowRef } from 'vue'

import type { OnboardingLocalizedContent } from '@/api/features'
import { useI18n } from '@/i18n'
import { newWelcomePair, normalizeContentID, welcomeContent, welcomePairs, type BilingualWelcome } from './onboardingEditor'

const props = defineProps<{ content: OnboardingLocalizedContent }>()
const emit = defineEmits<{ 'update:content': [content: OnboardingLocalizedContent] }>()
const { t } = useI18n()
const pairs = computed(() => welcomePairs(props.content))
const currentIndex = shallowRef<number | null>(null)
const selected = computed(() => currentIndex.value === null ? null : pairs.value[currentIndex.value])
const columns = computed(() => [{ accessorKey: 'id', header: t('adminOnboarding.identifier') }, { id: 'english', header: t('app.english') }, { id: 'chinese', header: t('app.simplifiedChinese') }, { id: 'actions', header: t('adminWorkspace.actions') }])
const canAdd = computed(() => pairs.value.length < 20)

function commit(next: BilingualWelcome[]): void {
  emit('update:content', welcomeContent(next))
}

function updateID(index: number, id: string): void {
  const normalizedID = normalizeContentID(id)
  commit(pairs.value.map((pair, position) => position === index
    ? { ...pair, id: normalizedID, english: { ...pair.english, id: normalizedID }, chinese: { ...pair.chinese, id: normalizedID } }
    : pair))
}

function updateText(index: number, locale: 'english' | 'chinese', text: string): void {
  commit(pairs.value.map((pair, position) => position === index
    ? { ...pair, [locale]: { ...pair[locale], text } }
    : pair))
}

function move(index: number, direction: -1 | 1): void {
  const target = index + direction
  if (target < 0 || target >= pairs.value.length) return
  const next = [...pairs.value]
  const current = next[index]
  next[index] = next[target]
  next[target] = current
  commit(next)
  currentIndex.value = target
}

function remove(index: number): void {
  if (pairs.value.length <= 1) return
  commit(pairs.value.filter((_, position) => position !== index))
  currentIndex.value = null
}

function add(): void {
  if (canAdd.value) { const index = pairs.value.length; commit([...pairs.value, newWelcomePair()]); currentIndex.value = index }
}
</script>

<template>
  <section class="content-index" :aria-label="t('adminOnboarding.welcome')">
    <UTable class="admin-table" :data="pairs" :columns="columns">
      <template #english-cell="{ row }"><span class="content-excerpt">{{ row.original.english.text }}</span></template>
      <template #chinese-cell="{ row }"><span class="content-excerpt">{{ row.original.chinese.text }}</span></template>
      <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminWorkspace.details')" @click="currentIndex = row.index" /><UButton color="neutral" variant="ghost" icon="i-ph-arrow-up" :disabled="row.index === 0" :aria-label="t('adminOnboarding.moveUp')" @click="move(row.index, -1)" /><UButton color="neutral" variant="ghost" icon="i-ph-arrow-down" :disabled="row.index === pairs.length - 1" :aria-label="t('adminOnboarding.moveDown')" @click="move(row.index, 1)" /><UButton color="error" variant="ghost" icon="i-ph-trash" :disabled="pairs.length <= 1" :aria-label="t('adminOnboarding.removeCard')" @click="remove(row.index)" /></div></template>
    </UTable>
    <UButton color="neutral" variant="outline" icon="i-ph-plus" :disabled="!canAdd" :label="t('adminOnboarding.addWelcome')" @click="add" />
    <USlideover :open="currentIndex !== null" :title="t('adminOnboarding.welcome')" :description="t('adminOnboarding.copy')" :ui="{ content: 'max-w-3xl' }" @update:open="!$event && (currentIndex = null)">
      <template #body>
        <div v-if="selected && currentIndex !== null" class="content-edit-workspace">
          <div class="admin-editor-form">
            <UFormField :label="t('adminOnboarding.identifier')">
              <UInput :model-value="selected.id" :maxlength="64" @update:model-value="updateID(currentIndex, String($event))" />
            </UFormField>
            <div class="welcome-card__locales">
              <UFormField :label="t('app.english')">
                <UTextarea :model-value="selected.english.text" :rows="4" :maxlength="1000" @update:model-value="updateText(currentIndex, 'english', String($event))" />
              </UFormField>
              <UFormField :label="t('app.simplifiedChinese')">
                <UTextarea :model-value="selected.chinese.text" :rows="4" :maxlength="1000" @update:model-value="updateText(currentIndex, 'chinese', String($event))" />
              </UFormField>
            </div>
          </div>
          <aside class="content-preview" :aria-label="t('adminWorkspace.overview')"><p>{{ selected.english.text }}</p><p>{{ selected.chinese.text }}</p></aside>
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
