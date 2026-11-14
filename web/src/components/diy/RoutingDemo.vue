<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import { useI18n } from '@/i18n'
import { useIllustrationPlayback } from '@/composables/useIllustrationPlayback'
import DemoTransport from '@/components/common/DemoTransport.vue'
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
const playback = useIllustrationPlayback({ active: () => open.value, stepCount: steps.length, interval: 5800 })
const { step, phase, progress, elapsed, duration, playing, reducedMotion } = playback
const overrideMatch = shallowRef<boolean | null>(null)
watch(open, () => { overrideMatch.value = null })
const matches = computed(() => overrideMatch.value ?? (step.value !== 3 || phase.value < .55))
const selected = computed(() => steps[step.value]!)
const description = computed(() => t(`diy.demo.descriptions.${selected.value.key}`))
const chapters = computed(() => steps.map(item => ({ icon: item.icon, label: t(`diy.demo.labels.${item.key}`) })))
const scenarios = computed(() => [
  { value: 'match', label: t('diy.demo.match'), icon: 'i-ph-check-circle' },
  { value: 'default', label: t('diy.demo.noMatch'), icon: 'i-ph-arrow-elbow-right' },
])
function scenario(value: string | number) { overrideMatch.value = value === 'match'; playback.selectStep(3) }
function replay() { overrideMatch.value = null; playback.replay() }
function restoreFocus(event: globalThis.Event): void {
  event.preventDefault()
  if (props.returnFocus?.isConnected) props.returnFocus.focus({ preventScroll: true })
}
</script>

<template>
  <UModal v-model:open="open" :title="t('diy.demo.title')" :description="t('diy.demo.description')" :content="{ onCloseAutoFocus: restoreFocus }" :transition="!reducedMotion" :close="{ 'aria-label': t('common.close') }" :ui="{ content: 'sm:max-w-xl', body: 'p-4 sm:p-6', description: 'sr-only' }">
    <template #body>
      <div class="routing-demo" :data-demo-playing="playing">
        <div class="routing-demo__example"><UIcon name="i-ph-flask" aria-hidden="true" />{{ t('diy.demo.example') }}</div>
        <RoutingDemoScene :step="step" :phase="phase" :matches="matches" :reduced-motion="reducedMotion" :description="description" />
        <p class="routing-demo__caption"><UIcon :name="selected.icon" aria-hidden="true" />{{ t(`diy.demo.captions.${selected.key}`) }}</p>
        <UTabs class="routing-demo__scenarios" :items="scenarios" :model-value="matches ? 'match' : 'default'" :content="false" size="sm" :aria-label="t('diy.demo.destination')" :ui="{ list: 'w-full', trigger: 'flex-1 min-h-10' }" @update:model-value="scenario" />
        <p class="sr-only" role="status" aria-live="polite" aria-atomic="true">{{ description }}</p>
        <DemoTransport :chapters="chapters" :step="step" :progress="progress" :elapsed="elapsed" :duration="duration" :playing="playing" :reduced-motion="reducedMotion" @select="playback.selectStep" @seek="playback.seek" @toggle="playback.toggle" @replay="replay" />
      </div>
    </template>
  </UModal>
</template>

<style scoped>
.routing-demo { display: grid; gap: .75rem; min-width: 0; }
.routing-demo__example { display: flex; align-items: center; gap: .35rem; color: var(--text-faint); font-size: .7rem; }
.routing-demo__caption { display: flex; align-items: center; justify-content: center; gap: .4rem; margin: 0; min-height: 2rem; color: var(--text); font-size: .8rem; text-align: center; }
.routing-demo__caption > .iconify { flex: none; color: var(--accent); font-size: 1.15rem; }
.routing-demo__scenarios { min-width: 0; }
</style>
