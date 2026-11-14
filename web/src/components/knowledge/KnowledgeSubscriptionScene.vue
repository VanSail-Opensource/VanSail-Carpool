<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoWindow from '@/components/common/DemoWindow.vue'
import DemoFlow from '@/components/common/DemoFlow.vue'
import { between, ease } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean }>()
const { t } = useI18n()
const p = computed(() => props.reducedMotion ? 1 : props.phase)
const done = computed(() => p.value >= .58)
const progress = computed(() => ease(between(p.value, .1, .8)))
const title = computed(() => props.step < 2 ? 'wallet' : props.step < 4 ? 'billingChoice' : props.step < 6 ? 'routingOptional' : props.step < 8 ? 'clientDownload' : props.step < 10 ? 'subscriptionLink' : 'connection')
const flow = computed(() => (props.step < 2 ? ['addBalance', 'invoice', 'balanceReady'] : ['entry', 'rules', 'exit']).map((key, index) => ({ label: t(`knowledge.labels.${key}`), icon: (props.step < 2 ? ['i-ph-wallet-bold', 'i-ph-receipt-bold', 'i-ph-check-circle-bold'] : ['i-ph-sign-in-bold', 'i-ph-funnel-bold', 'i-ph-sign-out-bold'])[index]! })))
const clients = [{ key: 'flclash', icon: 'i-ph-devices-bold' }, { key: 'v2rayn', icon: 'i-ph-desktop-bold' }, { key: 'v2rayng', icon: 'i-ph-device-mobile-bold' }]
const nodes = ['TX-01', 'TX-02', 'TX-03']
const connected = computed(() => props.step === 11 || (props.step === 10 && done.value))
</script>

<template>
  <DemoWindow class="subscription-scene" :title="t(`knowledge.labels.${title}`)" icon="i-ph-devices-bold" :status="t(`knowledge.labels.${connected ? 'connected' : 'tutorial'}`)" :ready="connected" :data-subscription-step="step">
    <div class="subscription-scene__screen">
      <template v-if="step < 2">
        <div class="subscription-balance"><span>{{ t(`knowledge.labels.${step === 1 && done ? 'balanceReady' : 'addBalance'}`) }}</span><strong>{{ step === 1 && done ? '10.00' : '0.00' }}<small>{{ 'TXB' }}</small></strong><span class="subscription-balance__badge" :style="{ opacity: step === 0 ? progress : 1, transform: `translateY(${step === 0 ? 8 * (1 - progress) : 0}px)` }"><UIcon :name="step === 1 && done ? 'i-ph-check-circle-fill' : 'i-ph-plus-bold'" />{{ '10.00' }}</span></div>
        <DemoFlow :nodes="flow" :phase="p" :active="step === 0 ? 0 : 2" :reduced-motion="reducedMotion" />
      </template>
      <template v-else-if="step === 2">
        <div class="subscription-options"><div v-for="(mode, index) in ['hourly', 'traffic']" :key="mode" :class="{ 'is-lit': reducedMotion || p >= .18 + index * .3 }"><UIcon :name="index === 0 ? 'i-ph-clock-bold' : 'i-ph-arrows-down-up-bold'" /><strong>{{ t(`knowledge.labels.${mode}`) }}</strong><span>{{ t(`knowledge.labels.${index === 0 ? 'hourlyPurchase' : 'trafficReady'}`) }}</span></div></div>
        <span class="subscription-note">{{ t('knowledge.labels.or') }}</span>
      </template>
      <template v-else-if="step === 3 || step === 9">
        <div class="subscription-label"><UIcon name="i-ph-hard-drives-bold" />{{ t('knowledge.labels.availableNodes') }}</div>
        <div class="subscription-nodes"><div v-for="(node, index) in nodes" :key="node" :style="{ opacity: .2 + .8 * between(p, index * .18, index * .18 + .2), transform: `translateY(${6 * (1 - between(p, index * .18, index * .18 + .2))}px)` }"><UIcon name="i-ph-hard-drives-bold" /><code>{{ node }}</code><UIcon name="i-ph-check-circle-fill" /></div></div>
      </template>
      <template v-else-if="step < 6">
        <div class="subscription-route"><span><UIcon name="i-ph-path-bold" />{{ t('knowledge.labels.defaultRoute') }}</span><span><UIcon name="i-ph-sliders-horizontal-bold" />{{ t('knowledge.labels.optional') }}</span></div>
        <DemoFlow :nodes="flow" :phase="p" :active="1" :reduced-motion="reducedMotion" />
        <div class="subscription-save" :class="{ 'is-ready': step === 5 && done }"><UIcon :name="step === 5 && done ? 'i-ph-check-circle-fill' : step === 5 ? 'i-ph-hourglass-bold' : 'i-ph-floppy-disk-bold'" />{{ t(`knowledge.labels.${step === 5 ? done ? 'applied' : 'pending' : 'save'}`) }}<i :style="{ transform: `scaleX(${step === 5 ? progress : 0})` }" /></div>
      </template>
      <template v-else-if="step < 8">
        <div class="subscription-clients"><div v-for="(client, index) in clients" :key="client.key" :class="{ 'is-lit': p >= index * .2 + .15 }"><UIcon :name="client.icon" /><span>{{ t(`knowledge.clients.${client.key}`) }}</span><UIcon :name="step === 7 ? 'i-ph-check-circle-fill' : 'i-ph-download-simple-bold'" /></div></div>
        <span class="subscription-note"><UIcon :name="step === 7 ? 'i-ph-shield-check-bold' : 'i-ph-link-bold'" />{{ t(`knowledge.labels.${step === 7 ? 'compatible' : 'official'}`) }}</span>
      </template>
      <template v-else-if="step === 8">
        <div class="subscription-label"><UIcon name="i-ph-lock-key-bold" />{{ t('knowledge.labels.privateLink') }}</div>
        <div class="subscription-input"><UIcon name="i-ph-link-bold" /><code>{{ `https://…/${'•'.repeat(Math.ceil(progress * 8))}` }}</code><UIcon :name="done ? 'i-ph-check-circle-fill' : 'i-ph-clipboard-text-bold'" /></div>
        <div class="subscription-save" :class="{ 'is-ready': done }"><UIcon :name="done ? 'i-ph-check-bold' : 'i-ph-floppy-disk-bold'" />{{ t(`knowledge.labels.${done ? 'saved' : 'save'}`) }}<i :style="{ transform: `scaleX(${progress})` }" /></div>
        <span class="subscription-note">{{ t('knowledge.labels.keepPrivate') }}</span>
      </template>
      <template v-else>
        <div class="subscription-connect" :class="{ 'is-ready': connected }"><div><UIcon name="i-ph-hard-drives-bold" /><code>{{ 'TX-01' }}</code><UIcon name="i-ph-check-circle-fill" /></div><span class="subscription-power"><UIcon name="i-ph-power-bold" :style="{ transform: `scale(${connected ? 1 : .92 + progress * .08})` }" /></span><strong>{{ t(`knowledge.labels.${connected ? 'connected' : 'connect'}`) }}</strong></div>
        <svg class="subscription-graph" viewBox="0 0 260 45" preserveAspectRatio="none"><path d="M0 40H260 M0 20H260" class="subscription-graph__grid" /><path d="M0 40 H20 L33 35 L45 38 L58 18 L70 27 L84 12 L99 20 L111 9 L123 22 L138 14 L155 20 L170 8 L186 18 L203 7 L219 15 L235 11 L260 13" class="subscription-graph__line" pathLength="1" :stroke-dasharray="`${connected ? progress : 0} 1`" /></svg>
      </template>
    </div>
    <template #footer><UIcon :name="step >= 8 ? 'i-ph-lock-key-bold' : 'i-ph-check-circle-bold'" />{{ t(`knowledge.labels.${step >= 8 && step < 10 ? 'privateLink' : step >= 10 ? 'status' : 'example'}`) }}</template>
  </DemoWindow>
</template>

<style scoped>
.subscription-scene__screen { display: grid; align-content: center; gap: .9rem; min-height: 225px; }
.subscription-balance { position: relative; display: grid; gap: .5rem; padding: .3rem .25rem .6rem; }
.subscription-balance > span:first-child { color: var(--text-muted); font-size: .7rem; }
.subscription-balance strong { color: var(--text); font: 600 2.25rem var(--font-mono); }
.subscription-balance strong small { padding-left: .45rem; color: var(--text-faint); font-size: .7rem; }
.subscription-balance__badge { position: absolute; top: .2rem; right: 0; display: flex; align-items: center; gap: .25rem; padding: .4rem .55rem; border-radius: .7rem; background: var(--accent-soft); color: var(--accent); font: .7rem var(--font-mono); }
.subscription-options { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .7rem; }
.subscription-options > div { display: grid; justify-items: center; gap: .7rem; min-height: 160px; align-content: center; padding: .8rem .55rem; border: 1px solid var(--line); border-radius: 1rem; color: var(--text-faint); text-align: center; }
.subscription-options > div > .iconify { font-size: 2rem; }
.subscription-options strong { color: var(--text); font-size: .86rem; }
.subscription-options span { font-size: .64rem; line-height: 1.4; }
.subscription-options .is-lit { border-color: var(--accent-line); color: var(--accent); background: var(--accent-soft); }
.subscription-note { display: flex; align-items: center; justify-content: center; gap: .4rem; color: var(--text-faint); font-size: .65rem; }
.subscription-label { display: flex; align-items: center; gap: .45rem; color: var(--text-muted); font-size: .7rem; }
.subscription-nodes, .subscription-clients { display: grid; gap: .6rem; }
.subscription-nodes > div, .subscription-clients > div { display: flex; align-items: center; gap: .6rem; padding: .7rem; border: 1px solid var(--line); border-radius: .8rem; color: var(--text-muted); font-size: .73rem; }
.subscription-nodes > div > .iconify:first-child, .subscription-clients > div > .iconify:first-child { flex: none; color: var(--accent); font-size: 1.1rem; }
.subscription-nodes > div > .iconify:last-child, .subscription-clients > div > .iconify:last-child { margin-left: auto; flex: none; color: var(--success); }
.subscription-clients > div:not(.is-lit) { opacity: .4; }
.subscription-route { display: flex; align-items: center; justify-content: space-between; gap: .6rem; }
.subscription-route > span { display: flex; align-items: center; gap: .35rem; color: var(--text-muted); font-size: .65rem; }
.subscription-route > span:last-child { color: var(--accent); }
.subscription-save { position: relative; display: flex; align-items: center; justify-content: center; gap: .5rem; min-height: 42px; overflow: hidden; border: 1px solid var(--line); border-radius: .85rem; color: var(--text-muted); font-size: .7rem; }
.subscription-save > i { position: absolute; bottom: 0; left: 0; right: 0; height: 3px; background: var(--accent); transform-origin: left; }
.subscription-save.is-ready { color: var(--success); background: var(--accent-soft); }
.subscription-input { display: flex; align-items: center; gap: .5rem; padding: .9rem .7rem; min-width: 0; border: 1px solid var(--accent-line); border-radius: .8rem; color: var(--accent); }
.subscription-input code { min-width: 0; color: var(--text); font-size: .8rem; overflow: hidden; white-space: nowrap; }
.subscription-input > .iconify:last-child { margin-left: auto; }
.subscription-connect { display: grid; justify-items: center; gap: .7rem; color: var(--text-faint); }
.subscription-connect > div { display: flex; align-items: center; gap: .55rem; color: var(--text-muted); font-size: .7rem; }
.subscription-power { display: grid; place-items: center; width: 70px; height: 70px; border: 1px solid var(--line); border-radius: 1.4rem; background: var(--surface-soft); font-size: 2rem; }
.subscription-connect.is-ready .subscription-power { color: var(--accent); background: var(--accent-soft); border-color: var(--accent-line); }
.subscription-connect strong { color: var(--text); font-size: .8rem; }
.subscription-graph { width: 100%; height: 45px; overflow: visible; }
.subscription-graph__grid { fill: none; stroke: var(--line); stroke-width: 1; }
.subscription-graph__line { fill: none; stroke: var(--accent); stroke-width: 2; }
@media (max-width: 380px) { .subscription-options { gap: .45rem; } .subscription-options strong { font-size: .75rem; } .subscription-clients > div { font-size: .65rem; } }
</style>
