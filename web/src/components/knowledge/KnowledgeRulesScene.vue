<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoFlow from '@/components/common/DemoFlow.vue'
import DemoWindow from '@/components/common/DemoWindow.vue'
import { between, ease } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean }>()
const { t } = useI18n()
const p = computed(() => props.reducedMotion ? 1 : props.phase)
const travel = computed(() => ease(between(p.value, .1, .84)))
const denied = computed(() => props.step >= 4 && props.step <= 6)
const privateLink = computed(() => props.step === 2 || props.step === 3)
const controls = computed(() => props.step === 7 || props.step === 8)
const limit = computed(() => Math.round(6 + 14 * travel.value))
const status = computed(() => props.step === 9 ? 'support' : denied.value ? 'prohibited' : privateLink.value ? 'keepPrivate' : controls.value ? 'controlled' : 'allowed')
const rows = computed(() => {
  if (props.step === 0) return [['i-ph-browser-bold', 'browsing'], ['i-ph-play-circle-bold', 'streaming'], ['i-ph-game-controller-bold', 'game']]
  if (props.step === 1) return [['i-ph-sparkle-bold', 'ai'], ['i-ph-briefcase-bold', 'work'], ['i-ph-shopping-bag-bold', 'commerce']]
  if (props.step === 4) return [['i-ph-robot-bold', 'crawler'], ['i-ph-users-three-bold', 'registration'], ['i-ph-share-network-bold', 'resharing']]
  if (props.step === 5) return [['i-ph-arrows-down-up-bold', 'torrent'], ['i-ph-envelope-bold', 'spam'], ['i-ph-shield-warning-bold', 'attacks']]
  return [['i-ph-bug-bold', 'malware'], ['i-ph-password-bold', 'credentialAttacks'], ['i-ph-gauge-bold', 'connectionFlood']]
})
const nodes = computed(() => (privateLink.value
  ? [['i-ph-user-circle-bold', 'you'], ['i-ph-lock-key-bold', 'privateLink'], ['i-ph-devices-bold', 'device']]
  : denied.value ? [['i-ph-warning-circle-bold', 'abuse'], ['i-ph-shield-check-bold', 'protocol'], ['i-ph-prohibit-bold', 'prohibited']]
    : controls.value ? [['i-ph-plugs-connected-bold', 'connection'], ['i-ph-shield-check-bold', 'protocol'], ['i-ph-gauge-bold', 'connections']]
      : props.step === 9 ? [['i-ph-warning-circle-bold', 'issue'], ['i-ph-chats-circle-bold', 'bot'], ['i-ph-magnifying-glass-bold', 'diagnose']]
        : [['i-ph-devices-bold', 'device'], ['i-ph-shield-check-bold', 'rules'], ['i-ph-globe-bold', 'service']])
  .map(([icon, label]) => ({ icon: icon!, label: t(`knowledge.labels.${label}`) })))
const reveal = (index: number) => ease(between(p.value, .08 + index * .17, .33 + index * .17))
const reportFields = [['i-ph-hard-drives-bold', 'nodeSelected'], ['i-ph-clock-bold', 'time'], ['i-ph-browser-bold', 'issue']] as const
</script>

<template>
  <DemoWindow class="rules-film" :title="t('knowledge.labels.rules')" icon="i-ph-shield-check-bold" :status="t(`knowledge.labels.${status}`)" :ready="!denied">
    <div class="rules-film__content" :class="{ 'is-denied': denied }">
      <div v-if="privateLink" class="rules-film__privacy">
        <span class="rules-film__lock" :style="{ transform: `scale(${.85 + .15 * travel})` }"><UIcon name="i-ph-lock-key-bold" /></span>
        <div class="rules-film__secret"><span><UIcon name="i-ph-link-bold" />{{ t('knowledge.labels.subscriptionLink') }}</span><div class="rules-film__mask"><i v-for="n in 12" :key="n" :style="{ opacity: .25 + .75 * between(p, (n - 1) * .04, .3 + n * .04) }" /></div></div>
        <span class="rules-film__privacy-note"><UIcon name="i-ph-eye-slash-bold" />{{ t('knowledge.labels.keepPrivate') }}</span>
      </div>
      <div v-else-if="controls" class="rules-film__controls">
        <div class="rules-film__count"><span>{{ t('knowledge.labels.connections') }}</span><strong>{{ limit }}<small>{{ ' / 20' }}</small></strong><UIcon :name="limit === 20 ? 'i-ph-shield-check-bold' : 'i-ph-plugs-connected-bold'" /></div>
        <div class="rules-film__capacity"><i v-for="n in 20" :key="n" :class="{ 'is-filled': n <= limit }" /></div>
        <div class="rules-film__guard"><UIcon :name="step === 8 ? 'i-ph-warning-circle-bold' : 'i-ph-shield-check-bold'" /><span>{{ t(`knowledgeLessons.rules.steps.${step === 8 ? 'limits' : 'controls'}.points.second`) }}</span></div>
      </div>
      <div v-else-if="step === 9" class="rules-film__report">
        <div v-for="(field, index) in reportFields" :key="field[1]" :style="{ opacity: .3 + .7 * reveal(index) }"><UIcon :name="field[0]" /><span>{{ t(`knowledge.labels.${field[1]}`) }}</span><i :style="{ transform: `scaleX(${reveal(index)})` }" /><UIcon name="i-ph-check-bold" /></div>
        <span class="rules-film__send" :style="{ opacity: .3 + .7 * between(p, .6, .9) }"><UIcon name="i-ph-paper-plane-tilt-bold" />{{ t('knowledge.labels.bot') }}</span>
      </div>
      <div v-else class="rules-film__activity">
        <div v-for="(row, index) in rows" :key="row[1]" class="rules-film__row" :class="{ 'is-checked': p >= .3 + index * .18 }"><span class="rules-film__app"><UIcon :name="row[0]" /></span><span>{{ t(`knowledge.labels.${row[1]}`) }}</span><i :style="{ transform: `scaleX(${reveal(index)})` }" /><UIcon :name="denied ? 'i-ph-prohibit-bold' : 'i-ph-check-circle-fill'" :style="{ opacity: reveal(index), transform: `scale(${.6 + .4 * reveal(index)})` }" /></div>
      </div>
    </div>
    <DemoFlow :nodes="nodes" :phase="p" :active="Math.min(2, Math.floor(travel * 3))" :blocked="denied" :reduced-motion="reducedMotion" />
  </DemoWindow>
</template>

<style scoped>
.rules-film__content { display: grid; align-items: center; min-height: 155px; }
.rules-film__activity, .rules-film__report { display: grid; gap: .6rem; }
.rules-film__row { display: flex; align-items: center; gap: .65rem; min-height: 2.5rem; color: var(--text-muted); font-size: .7rem; }
.rules-film__app { display: grid; place-items: center; width: 2.4rem; height: 2.4rem; border: 1px solid var(--line); border-radius: .75rem; color: var(--text); font-size: 1.2rem; }
.rules-film__row > i { height: 2px; flex: 1; background: var(--success); transform-origin: left; }
.rules-film__row > .iconify { color: var(--success); font-size: 1.15rem; }
.rules-film__row.is-checked .rules-film__app { border-color: color-mix(in srgb, var(--success) 45%, var(--line)); }
.is-denied .rules-film__row > i { background: var(--warning); }
.is-denied .rules-film__row > .iconify { color: var(--warning); }
.is-denied .rules-film__row.is-checked .rules-film__app { border-color: color-mix(in srgb, var(--warning) 45%, var(--line)); }
.rules-film__privacy { display: grid; grid-template-columns: 3.8rem minmax(0, 1fr); align-items: center; gap: 1rem; padding: .6rem; }
.rules-film__lock { display: grid; place-items: center; width: 3.8rem; height: 3.8rem; border-radius: 1.2rem; color: var(--accent); background: var(--accent-soft); font-size: 2rem; }
.rules-film__secret { display: grid; gap: .9rem; min-width: 0; }
.rules-film__secret > span { display: flex; align-items: center; gap: .4rem; color: var(--text-muted); font-size: .7rem; }
.rules-film__mask { display: flex; flex-wrap: wrap; gap: .28rem; }
.rules-film__mask i { width: .35rem; height: .35rem; border-radius: 50%; background: var(--text); }
.rules-film__privacy-note { grid-column: 1 / -1; display: flex; justify-content: center; gap: .4rem; color: var(--accent); font-size: .7rem; }
.rules-film__controls { display: grid; gap: 1rem; }
.rules-film__count { display: flex; align-items: center; gap: .6rem; font-size: .72rem; color: var(--text-muted); }
.rules-film__count strong { margin-inline-start: auto; color: var(--text); font: 1.8rem var(--font-mono); }
.rules-film__count small { font-size: .75rem; color: var(--text-faint); }
.rules-film__count .iconify { color: var(--accent); font-size: 1.4rem; }
.rules-film__capacity { display: flex; gap: .22rem; }
.rules-film__capacity i { flex: 1; height: 1.65rem; border-radius: .2rem; background: var(--line); }
.rules-film__capacity .is-filled { background: var(--accent); }
.rules-film__guard { display: flex; justify-content: center; align-items: center; gap: .4rem; color: var(--text-muted); font-size: .67rem; }
.rules-film__guard .iconify { color: var(--accent); font-size: 1.05rem; }
.rules-film__report > div { display: flex; align-items: center; gap: .5rem; min-height: 1.65rem; font-size: .67rem; color: var(--text-muted); }
.rules-film__report > div > .iconify { color: var(--accent); }
.rules-film__report > div > i { height: .25rem; flex: 1; border-radius: .2rem; background: var(--accent-line); transform-origin: left; }
.rules-film__send { display: flex; justify-content: center; align-items: center; gap: .45rem; min-height: 2.2rem; border-radius: .8rem; background: var(--accent-soft); color: var(--accent); font-size: .7rem; }
@media (max-width: 380px) { .rules-film__row { gap: .45rem; font-size: .65rem; } .rules-film__privacy { padding-inline: 0; gap: .65rem; } .rules-film__count strong { font-size: 1.5rem; } }
</style>
