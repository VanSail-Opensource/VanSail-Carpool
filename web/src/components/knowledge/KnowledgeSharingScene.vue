<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoWindow from '@/components/common/DemoWindow.vue'
import DemoFlow from '@/components/common/DemoFlow.vue'
import { between, ease } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean }>()
const { t } = useI18n()
const p = computed(() => props.reducedMotion ? 1 : props.phase)
const count = computed(() => props.step < 2 ? 6 : 3)
const blocked = computed(() => props.step === 5)
const checks = ['device', 'account', 'behavior']
const progress = computed(() => ease(between(p.value, .1, .82)))
const flow = computed(() => [
  { icon: 'i-ph-users-three-bold', label: t(`knowledge.labels.${props.step < 2 ? 'crowd' : 'members'}`) },
  { icon: 'i-ph-hard-drives-bold', label: props.step < 2 ? 'CGNAT' : t('knowledge.labels.dedicatedIp') },
  { icon: 'i-ph-browser-bold', label: t('knowledge.labels.service') },
])
</script>

<template>
  <DemoWindow class="sharing-scene" :title="t(`knowledge.labels.${step < 2 ? 'publicIp' : step < 6 ? 'controlled' : 'serviceChecks'}`)" icon="i-ph-users-three-bold">
    <div class="sharing-members"><span v-for="index in 6" :key="index" :class="{ 'is-member': index <= count, 'is-blocked': blocked && index > 3 }" :style="{ opacity: index <= count ? .3 + .7 * between(p, (index - 1) * .08, (index - 1) * .08 + .22) : .2 }"><UIcon :name="index <= count ? 'i-ph-user-circle-bold' : blocked ? 'i-ph-prohibit-bold' : 'i-ph-user-minus-bold'" /></span></div>
    <DemoFlow :nodes="flow" :phase="p" :active="step < 2 ? 1 : step < 6 ? 0 : 2" :blocked="blocked" :reduced-motion="reducedMotion" />
    <div class="sharing-detail">
      <template v-if="step < 4">
        <div class="sharing-ip"><UIcon name="i-ph-globe-bold" /><span><small>{{ t('knowledge.labels.publicIp') }}</small><code>{{ '203.0.113.8' }}</code></span><UIcon name="i-ph-users-three-bold" /></div>
        <div class="sharing-note"><UIcon :name="step === 3 ? 'i-ph-not-equals-bold' : 'i-ph-info-bold'" />{{ t(`knowledge.labels.${step === 3 ? 'shared' : step < 2 ? 'crowd' : 'dedicatedIp'}`) }}</div>
      </template>
      <template v-else-if="step < 6 || step === 8">
        <div class="sharing-limit"><span><UIcon name="i-ph-users-bold" />{{ t('knowledge.labels.limit') }}</span><strong>{{ '3 / 6' }}</strong></div>
        <div class="sharing-meter"><i v-for="index in 6" :key="index" :class="{ 'is-filled': index <= 3 }" :style="{ transform: `scaleY(${index <= 3 ? .15 + .85 * progress : .15})` }" /></div>
        <div class="sharing-gates"><span><UIcon name="i-ph-gauge-bold" />{{ t('knowledge.labels.connections') }}</span><span :class="{ 'is-blocked': blocked }"><UIcon :name="blocked ? 'i-ph-prohibit-bold' : 'i-ph-shield-check-bold'" />{{ t(`knowledge.labels.${blocked ? 'prohibited' : 'rules'}`) }}</span></div>
      </template>
      <template v-else-if="step < 9">
        <div class="sharing-checks"><span v-for="(key, index) in checks" :key="key" :class="{ 'is-lit': p >= index * .2 + .2 }"><UIcon :name="['i-ph-device-mobile-bold', 'i-ph-user-circle-bold', 'i-ph-fingerprint-bold'][index]" />{{ t(`knowledge.labels.${key}`) }}<UIcon name="i-ph-arrow-right-bold" /></span></div>
      </template>
      <template v-else>
        <div class="sharing-report"><UIcon name="i-ph-chats-circle-bold" /><span>{{ t('knowledge.labels.support') }}</span><UIcon :name="p >= .6 ? 'i-ph-check-circle-fill' : 'i-ph-paper-plane-tilt-bold'" :style="{ transform: `translateX(${(1 - progress) * -6}px)` }" /></div>
        <div class="sharing-note"><UIcon name="i-ph-lock-key-bold" />{{ t('knowledge.labels.noSecrets') }}</div>
      </template>
    </div>
    <template #footer><UIcon name="i-ph-scales-bold" />{{ t(`knowledge.labels.${step >= 6 ? 'noGuarantee' : 'shared'}`) }}</template>
  </DemoWindow>
</template>

<style scoped>
.sharing-members { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: .5rem; padding: .2rem .35rem; }
.sharing-members > span { display: grid; place-items: center; aspect-ratio: 1; border-radius: .8rem; background: var(--surface-soft); color: var(--text-faint); font-size: 1.55rem; }
.sharing-members > .is-member { color: var(--accent); background: var(--accent-soft); }
.sharing-members > .is-blocked { color: var(--warning, var(--text-muted)); }
.sharing-detail { display: grid; align-content: center; gap: .8rem; min-height: 130px; padding-top: .7rem; border-top: 1px solid var(--line); }
.sharing-ip { display: flex; align-items: center; gap: .7rem; padding: .7rem; border: 1px solid var(--accent-line); border-radius: .85rem; color: var(--accent); }
.sharing-ip > .iconify:first-child { font-size: 1.6rem; }
.sharing-ip > span { display: grid; gap: .3rem; }
.sharing-ip small { color: var(--text-faint); font-size: .6rem; }
.sharing-ip code { color: var(--text); font-size: .86rem; }
.sharing-ip > .iconify:last-child { margin-left: auto; }
.sharing-note { display: flex; align-items: center; justify-content: center; gap: .4rem; color: var(--text-faint); font-size: .65rem; }
.sharing-limit { display: flex; align-items: center; justify-content: space-between; gap: .6rem; }
.sharing-limit > span { display: flex; align-items: center; gap: .45rem; color: var(--text-muted); font-size: .65rem; }
.sharing-limit strong { color: var(--accent); font: 500 1.05rem var(--font-mono); }
.sharing-meter { display: grid; grid-template-columns: repeat(6, 1fr); align-items: end; gap: .45rem; height: 34px; }
.sharing-meter > i { height: 100%; border-radius: .35rem; background: var(--line); transform-origin: bottom; }
.sharing-meter > .is-filled { background: var(--accent); }
.sharing-gates { display: flex; flex-wrap: wrap; justify-content: space-between; gap: .5rem; }
.sharing-gates > span { display: flex; align-items: center; gap: .35rem; color: var(--text-muted); font-size: .65rem; }
.sharing-gates .is-blocked { color: var(--warning, var(--text-muted)); }
.sharing-checks { display: grid; gap: .6rem; }
.sharing-checks > span { display: flex; align-items: center; gap: .5rem; color: var(--text-faint); font-size: .7rem; }
.sharing-checks > .is-lit { color: var(--text); }
.sharing-checks > span > .iconify:first-child { color: var(--accent); }
.sharing-checks > span > .iconify:last-child { margin-left: auto; }
.sharing-report { display: flex; align-items: center; gap: .7rem; padding: .85rem; border: 1px solid var(--line); border-radius: 1rem; color: var(--text); font-size: .8rem; }
.sharing-report > .iconify:first-child { color: var(--accent); font-size: 1.6rem; }
.sharing-report > .iconify:last-child { margin-left: auto; color: var(--accent); }
@media (max-width: 380px) { .sharing-members { gap: .25rem; padding-inline: 0; } .sharing-members > span { font-size: 1.2rem; border-radius: .6rem; } }
</style>
