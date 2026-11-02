<script setup lang="ts">
import { computed } from 'vue'

import { squadFeatureIcon, squadFeatureIcons } from '@/components/squad-profile/featureIcons'
import { useI18n } from '@/i18n'
import { maxSquadFeatures, validSquadFeatures, type SquadFeature } from './featureForm'

const props = withDefaults(defineProps<{ showValidation?: boolean, busy?: boolean }>(), { showValidation: false, busy: false })
const features = defineModel<SquadFeature[]>({ required: true })
const { t } = useI18n()
const iconOptions = computed(() => Object.keys(squadFeatureIcons).map((value) => ({
  value, label: t(`adminSquad.features.icons.${value}`), icon: squadFeatureIcons[value as keyof typeof squadFeatureIcons],
})))

function update(index: number, patch: Partial<SquadFeature>): void {
  features.value = features.value.map((item, i) => i === index ? { ...item, ...patch } : item)
}

function move(index: number, direction: -1 | 1): void {
  const target = index + direction
  if (target < 0 || target >= features.value.length) return
  const reordered = [...features.value]
  ;[reordered[index], reordered[target]] = [reordered[target]!, reordered[index]!]
  features.value = reordered
}

function add(): void {
  if (features.value.length < maxSquadFeatures) features.value = [...features.value, { icon: 'route', title: '', detail: '' }]
}

function remove(index: number): void {
  features.value = features.value.filter((_, i) => i !== index)
}
</script>

<template>
  <section class="feature-editor" :aria-label="t('adminSquad.features.heading')">
    <div class="feature-editor__header">
      <div>
        <h3>{{ t('adminSquad.features.heading') }}</h3>
        <p>{{ t('adminSquad.features.hint') }}</p>
      </div>
    </div>
    <p v-if="!features.length" class="feature-editor__empty">{{ t('adminSquad.features.empty') }}</p>
    <ol v-else class="feature-editor__list">
      <li v-for="(feature, index) in features" :key="index" class="feature-editor__row">
        <div class="feature-editor__row-head">
          <div class="feature-editor__preview">
            <UIcon :name="squadFeatureIcon(feature.icon)" aria-hidden="true" />
            <div><strong>{{ feature.title.trim() || t('adminSquad.features.untitled') }}</strong><small v-if="feature.detail?.trim()">{{ feature.detail.trim() }}</small></div>
          </div>
          <div class="feature-editor__actions">
            <UButton type="button" color="neutral" variant="ghost" icon="i-ph-arrow-up" :aria-label="t('adminSquad.features.moveUp', { index: index + 1 })" :disabled="busy || index === 0" @click="move(index, -1)" />
            <UButton type="button" color="neutral" variant="ghost" icon="i-ph-arrow-down" :aria-label="t('adminSquad.features.moveDown', { index: index + 1 })" :disabled="busy || index === features.length - 1" @click="move(index, 1)" />
            <UButton type="button" color="neutral" variant="ghost" icon="i-ph-trash" :aria-label="t('adminSquad.features.remove', { index: index + 1 })" :disabled="busy" @click="remove(index)" />
          </div>
        </div>
        <div class="feature-editor__fields">
          <UFormField :name="`feature-icon-${index}`" :label="t('adminSquad.features.icon')">
            <USelect :model-value="feature.icon" :items="iconOptions" :disabled="busy" class="w-full" @update:model-value="update(index, { icon: $event as SquadFeature['icon'] })" />
          </UFormField>
          <UFormField :name="`feature-title-${index}`" :label="t('adminSquad.features.title')" required>
            <UInput :model-value="feature.title" :disabled="busy" @update:model-value="update(index, { title: String($event ?? '') })" />
          </UFormField>
          <UFormField class="feature-editor__detail" :name="`feature-detail-${index}`" :label="t('adminSquad.features.detail')">
            <UInput :model-value="feature.detail ?? ''" :disabled="busy" @update:model-value="update(index, { detail: String($event ?? '') })" />
          </UFormField>
        </div>
      </li>
    </ol>
    <UAlert v-if="props.showValidation && !validSquadFeatures(features)" color="warning" variant="soft" icon="i-ph-warning-circle" :description="t('adminSquad.features.invalid')" />
    <UButton type="button" color="neutral" variant="outline" icon="i-ph-plus" :label="t('adminSquad.features.add')" :disabled="busy || features.length >= maxSquadFeatures" @click="add" />
  </section>
</template>

<style scoped>
.feature-editor { display: grid; gap: .75rem; min-width: 0; padding-top: 1rem; border-top: 1px solid var(--line); }
.feature-editor__header, .feature-editor__row-head { display: flex; justify-content: space-between; align-items: start; gap: .75rem; }
.feature-editor__header h3 { margin: 0; font-size: .95rem; font-weight: 650; }
.feature-editor__header p, .feature-editor__empty { margin: .25rem 0 0; color: var(--text-muted); font-size: .8rem; line-height: 1.5; }
.feature-editor__list { display: grid; gap: .75rem; padding: 0; margin: 0; list-style: none; }
.feature-editor__row { display: grid; gap: .75rem; min-width: 0; padding: .75rem; border: 1px solid var(--line); border-radius: var(--radius-control); }
.feature-editor__preview { display: flex; align-items: center; gap: .5rem; min-width: 0; color: var(--text); font-size: .8rem; overflow-wrap: anywhere; }
.feature-editor__preview > span:first-child { flex: none; color: var(--accent); }
.feature-editor__preview strong, .feature-editor__preview small { display: block; }
.feature-editor__preview strong { font-weight: 600; }
.feature-editor__preview small { margin-top: .15rem; color: var(--text-muted); }
.feature-editor__actions { display: flex; flex: none; }
.feature-editor__fields { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 2fr); gap: .75rem; }
.feature-editor__detail { grid-column: 1 / -1; }
@media (max-width: 480px) { .feature-editor__fields { grid-template-columns: minmax(0, 1fr); } .feature-editor__detail { grid-column: auto; } }
</style>
