<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoStage from '@/components/common/DemoStage.vue'
import DemoNode from '@/components/common/DemoNode.vue'
import DemoFlow from '@/components/common/DemoFlow.vue'
import DemoPointer from '@/components/common/DemoPointer.vue'
import { between, interactionCue } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean; description: string }>()
const { t } = useI18n()
const progress = computed(() => props.reducedMotion ? .5 : (props.step + props.phase) * 1.4)
const selected = computed(() => props.step > 0 || props.phase >= .54)
const confirmed = computed(() => props.step === 3 && props.phase >= .3)
const exchange = computed(() => props.step === 2)
const feeVisible = computed(() => props.step === 1 || props.step === 2)
const checks = [
  { key: 'token', icon: 'i-ph-currency-circle-dollar', at: .18 },
  { key: 'network', icon: 'i-ph-circles-three-plus', at: .36 },
  { key: 'address', icon: 'i-ph-copy', at: .54 },
] as const
const cue = computed(() => interactionCue(props.phase, props.step === 0
  ? [{ at: .18, x: 20, y: 11 }, { at: .36, x: 50, y: 11 }, { at: .54, x: 80, y: 11 }]
  : props.step === 1 ? [{ at: .25, x: 50, y: 33 }]
    : props.step === 2 ? [{ at: .4, x: 81, y: 58 }]
      : [{ at: .62, x: 50, y: 80 }]))
</script>

<template>
  <DemoStage :description="description" class="payment-scene" :data-knowledge-step="step">
    <div class="payment-scene__checks">
      <span v-for="check in checks" :key="check.key" :class="{ 'is-checked': step > 0 || phase >= check.at }"><UIcon :name="check.icon" />{{ t(`knowledge.labels.${check.key}`) }}<UIcon name="i-ph-check-circle-fill" class="payment-scene__tick" /></span>
    </div>
    <DemoFlow path="M 18 53 H 82" :progress="progress" :active="selected" />
    <div class="payment-scene__fee" :style="{ opacity: feeVisible ? 1 : .15 }">
      <DemoFlow path="M 18 53 Q 24 29 50 29" :progress="progress" :active="feeVisible" tone="warning" />
      <DemoNode icon="i-ph-gas-pump" :label="t(`knowledge.labels.${exchange ? 'fee' : 'networkFee'}`)" :detail="exchange ? '0.200000 USDT' : undefined" :x="50" :y="32" :active="feeVisible" tone="warning" />
    </div>
    <DemoNode :icon="exchange ? 'i-ph-bank' : 'i-ph-wallet'" :label="t(`knowledge.labels.${exchange ? 'withdrawal' : 'wallet'}`)" :detail="exchange ? '12.545678 USDT' : '12.345678 USDT'" :x="18" :y="56" :active="selected" />
    <DemoNode :icon="confirmed ? 'i-ph-check-circle-fill' : 'i-ph-receipt'" :label="t(`knowledge.labels.${confirmed ? 'confirmations' : 'recipient'}`)" detail="12.345678 USDT" :x="82" :y="56" :active="selected" />
    <div class="payment-scene__amount" :style="{ opacity: step < 3 ? 1 : 0 }"><UIcon name="i-ph-equals" /><span>{{ t('knowledge.labels.exactAmount') }}</span></div>
    <div class="payment-scene__summary" :style="{ opacity: step === 3 ? 0 : 1 }">
      <UIcon :name="exchange ? 'i-ph-minus-circle' : 'i-ph-check-circle'" />
      <span>{{ t(`knowledge.labels.${exchange ? 'received' : 'gasSeparate'}`) }}</span>
      <strong v-if="exchange">12.345678</strong>
    </div>
    <div class="payment-scene__support" :style="{ opacity: step === 3 ? between(phase, .32, .5) : 0 }">
      <UIcon name="i-ph-chats-circle" /><span>{{ t('knowledge.labels.bot') }}</span><UIcon name="i-ph-hash" /><small>{{ t('knowledge.labels.txid') }}</small>
    </div>
    <span class="payment-scene__example">{{ t('knowledge.labels.example') }}</span>
    <DemoPointer :cue="cue" :reduced-motion="reducedMotion" />
  </DemoStage>
</template>

<style scoped>
.payment-scene__checks { position: absolute; inset: 4% 6% auto; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: .45rem; }
.payment-scene__checks > span { display: flex; align-items: center; justify-content: center; gap: .3rem; padding: .4rem .15rem; color: var(--text-faint); font-size: .65rem; border: 1px solid var(--line); border-radius: .5rem; }
.payment-scene__checks .is-checked { color: var(--accent); border-color: color-mix(in srgb, var(--accent) 40%, var(--line)); }
.payment-scene__tick { opacity: 0; font-size: .7rem; flex: none; }
.is-checked .payment-scene__tick { opacity: 1; }
.payment-scene__fee { position: absolute; inset: 0; transition: opacity .3s; }
.payment-scene__amount { position: absolute; top: 53%; left: 50%; transform: translate(-50%, -50%); display: grid; justify-items: center; gap: .1rem; padding: .2rem .4rem; background: var(--surface); color: var(--accent); font-size: .6rem; text-align: center; }
.payment-scene__amount > .iconify { font-size: 1.3rem; }
.payment-scene__summary, .payment-scene__support { position: absolute; inset-inline: 5%; top: 80%; display: flex; align-items: center; justify-content: center; gap: .4rem; color: var(--text-muted); font-size: .7rem; }
.payment-scene__summary > strong { font: .7rem var(--font-mono); color: var(--accent); }
.payment-scene__support > .iconify:first-child { font-size: 1.35rem; color: var(--accent); }
.payment-scene__support > small { font-size: .62rem; }
.payment-scene__example { position: absolute; bottom: 3%; left: 5%; color: var(--text-faint); font-size: .6rem; }
@media (prefers-reduced-motion: reduce) { .payment-scene__fee { transition: none; } }
</style>
