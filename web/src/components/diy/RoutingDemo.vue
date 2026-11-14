<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import { useIllustrationPlayback } from '@/composables/useIllustrationPlayback'
import RoutingDemoScene from './RoutingDemoScene.vue'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ returnFocus?: globalThis.HTMLElement | null }>()
const { t } = useI18n()
const steps = [
  { key: 'choose', icon: 'i-ph-sliders-horizontal' },
  { key: 'check', icon: 'i-ph-shield-check' },
  { key: 'save', icon: 'i-ph-floppy-disk' },
  { key: 'route', icon: 'i-ph-path' },
] as const
const playback = useIllustrationPlayback({ active: () => open.value, stepCount: steps.length, interval: 3200 })
const matches = shallowRef(true)
const selected = computed(() => steps[playback.step.value]!)
const description = computed(() => t(`diy.demo.descriptions.${selected.value.key}`))
const scenarios = computed(() => [
  { value: 'match', label: t('diy.demo.match'), icon: 'i-ph-check-circle' },
  { value: 'default', label: t('diy.demo.noMatch'), icon: 'i-ph-minus-circle' },
])
function scenario(value: string | number) { matches.value = value === 'match'; playback.selectStep(3) }
function restoreFocus(event: globalThis.Event): void {
  event.preventDefault()
  if (props.returnFocus?.isConnected) props.returnFocus.focus({ preventScroll: true })
}
</script>

<template>
  <UModal v-model:open="open" :title="t('diy.demo.title')" :description="t('diy.demo.description')" :content="{ onCloseAutoFocus: restoreFocus }" :close="{ 'aria-label': t('common.close') }" :ui="{ content: 'sm:max-w-xl', body: 'p-4 sm:p-6', description: 'sr-only' }">
    <template #body>
      <div class="routing-demo">
        <nav class="routing-demo-steps" :aria-label="t('diy.demo.steps')">
          <UButton v-for="(item, index) in steps" :key="item.key" type="button" color="neutral" variant="ghost" class="routing-demo-step" :class="{ 'is-current': index === playback.step.value, 'is-complete': index < playback.step.value }" :aria-current="index === playback.step.value ? 'step' : undefined" :aria-label="t(`diy.demo.labels.${item.key}`)" @click="playback.selectStep(index)">
            <span class="routing-demo-step-icon"><UIcon :name="item.icon" aria-hidden="true" /></span>
            <span>{{ t(`diy.demo.labels.${item.key}`) }}</span>
          </UButton>
        </nav>
        <RoutingDemoScene :step="playback.step.value" :matches="matches" :playing="playback.playing.value" :description="description" />
        <div class="routing-demo-scenario">
          <UTabs v-if="playback.step.value === 3" :items="scenarios" :model-value="matches ? 'match' : 'default'" :content="false" size="sm" :aria-label="t('diy.demo.destination')" :ui="{ list: 'w-full', trigger: 'flex-1 min-h-10' }" @update:model-value="scenario" />
          <p v-else class="routing-demo-caption">{{ t(`diy.demo.captions.${selected.key}`) }}</p>
        </div>
        <div class="routing-demo-controls">
          <span class="routing-demo-note"><UIcon name="i-ph-flask" aria-hidden="true" />{{ t('diy.demo.example') }}</span>
          <div class="routing-demo-buttons">
            <UButton v-if="!playback.reducedMotion.value" :icon="playback.playing.value ? 'i-ph-pause' : 'i-ph-play'" color="neutral" variant="soft" :aria-label="t(playback.playing.value ? 'diy.demo.pause' : 'diy.demo.play')" :title="t(playback.playing.value ? 'diy.demo.pause' : 'diy.demo.play')" @click="playback.toggle" />
            <UButton icon="i-ph-arrow-counter-clockwise" color="neutral" variant="ghost" :aria-label="t('diy.demo.replay')" :title="t('diy.demo.replay')" @click="playback.replay" />
            <UButton icon="i-ph-arrow-right" color="neutral" variant="ghost" :aria-label="t('diy.demo.next')" :title="t('diy.demo.next')" :disabled="playback.step.value === steps.length - 1" @click="playback.selectStep(playback.step.value + 1)" />
          </div>
        </div>
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.routing-demo { display: grid; gap: 1.25rem; min-width: 0; }
.routing-demo-steps { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: .35rem; }
.routing-demo-step { display: grid; justify-items: center; gap: .5rem; min-width: 0; padding: .6rem .2rem; font-size: .75rem; color: var(--text-muted); border-radius: .75rem; cursor: pointer; }
.routing-demo-step-icon { display: grid; place-items: center; width: 2.5rem; height: 2.5rem; font-size: 1.25rem; border: 1px solid var(--line); border-radius: .75rem; }
.routing-demo-step.is-current { color: var(--ui-text); background: var(--surface-raised); }
.routing-demo-step.is-current .routing-demo-step-icon { border-color: var(--ui-primary); color: var(--ui-primary); }
.routing-demo-step.is-complete .routing-demo-step-icon { color: var(--ui-success); }
.routing-demo-step:focus-visible { outline: 2px solid var(--ui-primary); outline-offset: 2px; }
.routing-demo-scenario { display: grid; align-items: center; min-height: 2.5rem; }
.routing-demo-caption { margin: 0; text-align: center; color: var(--text-muted); font-size: .8rem; }
.routing-demo-controls { display: flex; align-items: center; justify-content: space-between; gap: .5rem; }
.routing-demo-note { display: inline-flex; align-items: center; gap: .4rem; font-size: .75rem; color: var(--text-muted); }
.routing-demo-buttons { display: flex; gap: .25rem; }
@media (prefers-reduced-motion: no-preference) {
  .routing-demo-step, .routing-demo-step-icon { transition: background-color .2s, color .2s, border-color .2s; }
}
</style>
