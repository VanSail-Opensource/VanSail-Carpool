<script setup lang="ts">
import { computed, reactive, shallowRef, watch } from 'vue'
import type { LuckyDrawAdmin, LuckyDrawWrite } from '@/api/features'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
import AdminDrawPrizeRow from './AdminDrawPrizeRow.vue'
import AdminDrawForecast from './AdminDrawForecast.vue'
import { blankPrize, draftFromDraw, serializeDraw, type PrizeDraft } from './drawDraft'

const props = defineProps<{
  draw: LuckyDrawAdmin | null
  busy: boolean
  error?: string | null
  comboItems: { value: string; label: string }[]
  squadItems: { value: string; label: string }[]
}>()
const emit = defineEmits<{ save: [value: LuckyDrawWrite]; cancel: [] }>()
const { t } = useI18n()
const draft = reactive(draftFromDraw(props.draw))
const validationError = shallowRef<string | null>(null)
const kindItems = computed(() => [
  { value: 'instant', label: t('adminDrawRedesign.instant') },
  { value: 'raffle', label: t('adminDrawRedesign.raffle') },
])
const probabilityTotal = computed(() => draft.prizes.reduce((sum, prize) => sum + Math.round(Number(prize.probability) * 100), 0) / 100)
const stockTotal = computed(() => draft.prizes.reduce((sum, prize) => sum + Number(prize.stock), 0))
const sumsValid = computed(() => draft.kind === 'instant' ? probabilityTotal.value === 100 : stockTotal.value === draft.threshold)

watch(() => props.draw, (draw) => {
  Object.assign(draft, draftFromDraw(draw))
  validationError.value = null
})

function changePrize(index: number, patch: Partial<PrizeDraft>): void {
  Object.assign(draft.prizes[index]!, patch)
}
function save(): void {
  validationError.value = null
  if (!sumsValid.value) { validationError.value = t('adminDrawRedesign.sumError'); return }
  const body = serializeDraw(draft)
  if (!body) { validationError.value = t('adminDrawRedesign.invalid'); return }
  emit('save', body)
}
</script>

<template>
  <USlideover :open="true" :title="draw ? t('adminLuckyDraw.edit') : t('adminLuckyDraw.new')" :description="t('adminDrawRedesign.drawsHint')" :dismissible="!busy" :ui="{ content: 'max-w-3xl' }" @update:open="!$event && emit('cancel')">
    <template #body>
      <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      <form class="draw-editor" @submit.prevent="save">
        <div class="draw-editor__heading">
          <div>
            <h3>{{ draw ? t('adminLuckyDraw.edit') : t('adminLuckyDraw.new') }}</h3>
            <p>{{ draft.kind === 'raffle' ? t('adminDrawRedesign.raffleHint') : t('adminDrawRedesign.instantHint') }}</p>
          </div>
        </div>
        <div class="draw-editor__fields">
          <p v-if="draft.kind === 'raffle'" class="draw-editor__wide draw-editor__help">
            {{ t('adminDrawRedesign.groupAccessHint') }}
          </p>
          <UFormField :label="t('adminDrawRedesign.kind')" required>
            <USelect v-model="draft.kind" class="w-full" :items="kindItems" :disabled="Boolean(draw)" />
          </UFormField>
          <UFormField :label="t('adminDrawRedesign.name')" required>
            <UInput v-model.trim="draft.name" class="w-full" :maxlength="80" />
          </UFormField>
          <TxbAmountField id="lucky-draw-fee" v-model="draft.fee" :label="t('adminDrawRedesign.price')" min-minor="1" required />
          <UFormField v-if="draft.kind === 'instant'" :label="t('adminDrawRedesign.expected')" required>
            <UInput v-model.number="draft.expectedParticipation" class="w-full" type="number" :min="1" :max="1000000" :step="1" />
          </UFormField>
          <UFormField v-else :label="t('adminDrawRedesign.threshold')" required>
            <UInput v-model.number="draft.threshold" class="w-full" type="number" :min="1" :max="1000" :step="1" />
          </UFormField>
          <UFormField v-if="draft.kind === 'raffle'" :label="t('adminDrawRedesign.keyword')" required>
            <UInput v-model.trim="draft.keyword" class="w-full" :maxlength="64" />
          </UFormField>
          <UFormField v-if="draft.kind === 'raffle'" :label="t('adminDrawRedesign.command')" required>
            <UInput v-model.trim="draft.command" class="w-full" :maxlength="32" />
          </UFormField>
          <UFormField class="draw-editor__wide" :label="t('adminDrawRedesign.description')">
            <UTextarea v-model.trim="draft.description" class="w-full" :rows="2" :maxlength="300" />
          </UFormField>
          <UFormField v-if="draft.kind === 'instant'" :label="t('adminDrawRedesign.available')">
            <USwitch v-model="draft.enabled" />
          </UFormField>
        </div>
        <section class="draw-editor__prizes">
          <div class="draw-editor__section-heading">
            <div>
              <h4>{{ t('adminDrawRedesign.prizes') }}</h4>
              <p :class="{ 'draw-editor__sum--invalid': !sumsValid }">
                {{ draft.kind === 'instant'
                  ? t('adminDrawRedesign.probabilityTotal', { value: Number.isFinite(probabilityTotal) ? probabilityTotal.toFixed(2) : '—' })
                  : t('adminDrawRedesign.stockTotal', { value: Number.isFinite(stockTotal) ? stockTotal : '—', threshold: draft.threshold }) }}
              </p>
            </div>
            <UButton
              icon="i-ph-plus" color="neutral" variant="outline" :label="t('adminDrawRedesign.addPrize')"
              @click="draft.prizes.push(blankPrize())"
            />
          </div>
          <AdminDrawPrizeRow
            v-for="(prize, index) in draft.prizes" :key="prize.clientId" :prize="prize"
            :draw-kind="draft.kind" :combo-items="comboItems" :squad-items="squadItems"
            :removable="draft.prizes.length > 1" @change="changePrize(index, $event)"
            @remove="draft.prizes.splice(index, 1)"
          />
        </section>
        <UAlert v-if="validationError" color="error" variant="soft" icon="i-ph-warning" :description="validationError" />
        <div class="draw-editor__actions">
          <UButton color="neutral" variant="outline" :label="t('common.cancel')" @click="emit('cancel')" />
          <UButton type="submit" :loading="busy" :disabled="busy || !sumsValid" :label="t('adminDrawRedesign.save')" />
        </div>
        <AdminDrawForecast v-if="draw" :key="draw.updatedAt" :draw-id="draw.id" />
      </form>
    </template>
  </USlideover>
</template>

<style scoped>
.draw-editor { display: grid; gap: 1.2rem; padding: 0; }
.draw-editor__heading h3, .draw-editor__heading p, .draw-editor__section-heading h4, .draw-editor__section-heading p { margin: 0; }
.draw-editor__heading p, .draw-editor__section-heading p { margin-top: 0.3rem; color: var(--text-muted); font-size: 0.8rem; }
.draw-editor__fields { display: grid; gap: 0.8rem; grid-template-columns: minmax(0, 1fr); }
.draw-editor__prizes { display: grid; }
.draw-editor__section-heading { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 0.8rem; }
.draw-editor__sum--invalid { color: var(--danger); }
.draw-editor__help { margin: 0; color: var(--text-muted); font-size: 0.76rem; }
.draw-editor__actions { display: flex; justify-content: flex-end; gap: 0.6rem; }
@media (min-width: 680px) { .draw-editor__fields { grid-template-columns: repeat(2, minmax(0, 1fr)); } .draw-editor__wide { grid-column: 1 / -1; } }
</style>
