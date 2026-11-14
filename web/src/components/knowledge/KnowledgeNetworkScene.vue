<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoWindow from '@/components/common/DemoWindow.vue'
import DemoFlow from '@/components/common/DemoFlow.vue'
import { between, ease } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean }>()
const { t } = useI18n()
const p = computed(() => props.reducedMotion ? 1 : props.phase)
const progress = computed(() => ease(between(p.value, .08, .85)))
const category = computed(() => props.step < 2 ? 0 : props.step < 4 ? 1 : props.step < 6 ? 2 : -1)
const types = [{ key: 'optimized', icon: 'i-ph-lightning-bold' }, { key: 'international', icon: 'i-ph-globe-hemisphere-west-bold' }, { key: 'broadband', icon: 'i-ph-house-line-bold' }]
const flow = computed(() => [
  { icon: 'i-ph-devices-bold', label: t('knowledge.labels.you') },
  { icon: 'i-ph-lightning-bold', label: t('knowledge.labels.entry') },
  { icon: props.step >= 4 ? 'i-ph-house-line-bold' : 'i-ph-globe-hemisphere-west-bold', label: t('knowledge.labels.exit') },
  { icon: 'i-ph-browser-bold', label: t('knowledge.labels.service') },
])
const checks = computed(() => props.step === 3 ? ['reputation', 'behavior', 'serviceDecision'] : ['device', 'account', 'serviceDecision'])
const bars = [34, 68, 112]
</script>

<template>
  <DemoWindow class="network-scene" :title="t('knowledge.labels.network')" icon="i-ph-path-bold">
    <div class="network-types"><span v-for="(type, index) in types" :key="type.key" :class="{ 'is-selected': index === category || (step >= 6 && index !== 1) }"><UIcon :name="type.icon" />{{ t(`knowledge.labels.${type.key}`) }}</span></div>
    <DemoFlow :nodes="flow" :phase="p" :active="step < 2 ? 1 : step < 7 ? 2 : 3" :reduced-motion="reducedMotion" />
    <div class="network-detail">
      <template v-if="step < 2 || step === 8">
        <div class="network-detail__heading"><UIcon name="i-ph-gauge-bold" />{{ t(`knowledge.labels.${step === 8 ? 'compare' : 'latency'}`) }}<small>{{ t('knowledge.labels.example') }}</small></div>
        <div class="network-bars"><div v-for="(value, index) in bars" :key="value"><UIcon :name="types[index]!.icon" /><span><i :style="{ transform: `scaleX(${value / 120 * progress})` }" /></span><code>{{ Math.round(value * progress) }} {{ 'ms' }}</code></div></div>
      </template>
      <template v-else-if="step === 2">
        <div class="network-detail__heading"><UIcon name="i-ph-globe-bold" />{{ t('knowledge.labels.globalReach') }}</div>
        <div class="network-regions"><span v-for="(icon, index) in ['i-ph-globe-hemisphere-east-bold', 'i-ph-globe-hemisphere-west-bold', 'i-ph-globe-bold']" :key="icon" :style="{ opacity: .25 + .75 * between(p, index * .2, index * .2 + .2), transform: `translateY(${8 * (1 - between(p, index * .2, index * .2 + .2))}px)` }"><UIcon :name="icon" /><UIcon name="i-ph-path-bold" /></span></div>
      </template>
      <template v-else-if="step === 3 || step === 5 || step === 7">
        <div class="network-detail__heading"><UIcon name="i-ph-fingerprint-bold" />{{ t('knowledge.labels.serviceChecks') }}</div>
        <div class="network-checks"><span v-for="(key, index) in checks" :key="key" :class="{ 'is-checked': p >= .2 + index * .2 }"><UIcon :name="['i-ph-device-mobile-bold', 'i-ph-user-circle-bold', 'i-ph-shield-check-bold'][index]" />{{ t(`knowledge.labels.${key}`) }}<UIcon :name="index === 2 ? 'i-ph-scales-bold' : 'i-ph-check-circle-bold'" /></span></div>
      </template>
      <template v-else>
        <div class="network-pair"><span><UIcon name="i-ph-lightning-bold" />{{ t('knowledge.labels.entry') }}</span><UIcon name="i-ph-plus-bold" /><span><UIcon name="i-ph-house-line-bold" />{{ t('knowledge.labels.localIsp') }}</span></div>
        <div class="network-route"><UIcon name="i-ph-path-bold" /><i><b :style="{ transform: `scaleX(${progress})` }" /></i><UIcon :name="p > .7 ? 'i-ph-check-circle-fill' : 'i-ph-browser-bold'" /></div>
      </template>
    </div>
    <template #footer><UIcon name="i-ph-info-bold" />{{ t(`knowledge.labels.${step < 3 ? 'congestion' : 'noGuarantee'}`) }}</template>
  </DemoWindow>
</template>

<style scoped>
.network-types { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: .45rem; }
.network-types > span { display: grid; justify-items: center; gap: .4rem; padding: .65rem .25rem; border: 1px solid var(--line); border-radius: .85rem; color: var(--text-faint); font-size: .62rem; text-align: center; overflow-wrap: anywhere; }
.network-types .iconify { font-size: 1.2rem; }
.network-types .is-selected { color: var(--accent); border-color: var(--accent-line); background: var(--accent-soft); }
.network-detail { display: grid; align-content: center; gap: .7rem; min-height: 126px; border-top: 1px solid var(--line); padding-top: .7rem; }
.network-detail__heading { display: flex; align-items: center; gap: .4rem; color: var(--text-muted); font-size: .67rem; }
.network-detail__heading > small { margin-left: auto; color: var(--text-faint); font-size: .58rem; }
.network-bars { display: grid; gap: .6rem; }
.network-bars > div { display: grid; grid-template-columns: 1rem minmax(0, 1fr) 3rem; align-items: center; gap: .6rem; color: var(--text-muted); }
.network-bars > div > span { height: 7px; overflow: hidden; border-radius: 7px; background: var(--surface-soft); }
.network-bars i { display: block; height: 100%; border-radius: 7px; background: var(--accent); transform-origin: left; }
.network-bars > div:nth-child(2) i { opacity: .65; }
.network-bars > div:nth-child(3) i { opacity: .4; }
.network-bars code { color: var(--text); font-size: .65rem; text-align: end; }
.network-regions { display: grid; grid-template-columns: repeat(3, 1fr); gap: .6rem; }
.network-regions > span { display: grid; justify-items: center; gap: .6rem; color: var(--accent); padding-block: .5rem; }
.network-regions > span > .iconify:first-child { font-size: 2.6rem; }
.network-regions > span > .iconify:last-child { color: var(--text-faint); }
.network-checks { display: grid; gap: .4rem; }
.network-checks > span { display: flex; align-items: center; gap: .45rem; color: var(--text-faint); font-size: .66rem; }
.network-checks > span > .iconify:last-child { margin-left: auto; }
.network-checks > .is-checked { color: var(--text-muted); }
.network-checks > .is-checked > .iconify:last-child { color: var(--accent); }
.network-pair { display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; gap: .6rem; color: var(--text-faint); }
.network-pair > span { display: grid; justify-items: center; gap: .6rem; color: var(--text-muted); font-size: .7rem; }
.network-pair > span > .iconify { color: var(--accent); font-size: 2rem; }
.network-route { display: flex; align-items: center; gap: .65rem; padding: .5rem 1rem; color: var(--accent); }
.network-route > i { flex: 1; height: 3px; background: var(--line); overflow: hidden; }
.network-route b { display: block; height: 3px; background: var(--accent); transform-origin: left; }
@media (max-width: 380px) { .network-types { gap: .25rem; } .network-types > span { font-size: .56rem; } }
</style>
