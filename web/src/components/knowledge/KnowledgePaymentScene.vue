<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import DemoWindow from '@/components/common/DemoWindow.vue'
import DemoFlow from '@/components/common/DemoFlow.vue'
import { between, ease } from '@/components/common/illustrationTimeline'

const props = defineProps<{ step: number; phase: number; reducedMotion: boolean }>()
const { t } = useI18n()
const p = computed(() => props.reducedMotion ? 1 : props.phase)
const mode = computed(() => props.step === 3 ? (p.value < .5 ? 'deducted' : 'separate') : [4, 5].includes(props.step) ? 'deducted' : [6, 7].includes(props.step) ? 'wallet' : 'invoice')
// Exact illustrative decimal strings, never input to payment/account code.
const amount = '12.345678'
const fee = '0.200000'
const transfer = computed(() => mode.value === 'deducted' ? '12.545678' : amount)
const checked = computed(() => p.value >= .55)
const flow = computed(() => ['wallet', 'network', 'recipient'].map((key, index) => ({ label: t(`knowledge.labels.${key}`), icon: ['i-ph-wallet-bold', 'i-ph-circles-three-plus-bold', 'i-ph-check-circle-bold'][index]! })))
const confirmation = computed(() => ['network', 'confirmations', 'credited'].map((key, index) => ({ label: t(`knowledge.labels.${key}`), icon: ['i-ph-broadcast-bold', 'i-ph-check-circle-bold', 'i-ph-wallet-bold'][index]! })))
const progress = computed(() => ease(between(p.value, .1, .8)))
</script>

<template>
  <DemoWindow
    class="knowledge-payment" :title="t(`knowledge.labels.${step < 3 ? 'invoice' : step < 6 ? 'withdrawal' : step < 8 ? 'wallet' : step < 10 ? 'received' : 'support'}`)" icon="i-ph-receipt-bold" :status="t(`knowledge.labels.${step === 9 ? 'credited' : 'example'}`)" :ready="step === 9"
    :data-payment-mode="mode" :data-payment-amount="amount" :data-payment-fee="fee" :data-payment-transfer="transfer" :data-payment-received="amount"
  >
    <div class="payment-invoice"><span>{{ t('knowledge.labels.displayedAmount') }}</span><strong>{{ amount }}<small>{{ 'USDT' }}</small></strong><UIcon name="i-ph-receipt-bold" /></div>
    <div class="payment-demo__content">
      <template v-if="step < 3">
        <div class="payment-match"><span><UIcon name="i-ph-coins-bold" />{{ 'USDT' }}</span><UIcon name="i-ph-equals-bold" /><span><UIcon name="i-ph-coins-bold" />{{ 'USDT' }}</span></div>
        <div class="payment-match" :class="{ 'is-checked': checked }"><span><UIcon name="i-ph-git-branch-bold" />{{ 'TRON' }}</span><UIcon :name="checked ? 'i-ph-check-circle-fill' : 'i-ph-equals-bold'" /><span><UIcon name="i-ph-git-branch-bold" />{{ 'TRON' }}</span></div>
        <div v-if="step === 2" class="payment-deadline"><UIcon name="i-ph-timer-bold" /><span>{{ t('knowledge.labels.deadline') }}</span><b>{{ `09:${String(59 - Math.floor(p * 3)).padStart(2, '0')}` }}</b><i :style="{ transform: `scaleX(${1 - progress * .3})` }" /></div>
        <div v-else class="payment-address"><UIcon name="i-ph-qr-code-bold" /><span>{{ t('knowledge.labels.address') }}<code>{{ 'T…••••…' }}</code></span><UIcon :name="checked ? 'i-ph-check-circle-fill' : 'i-ph-copy-bold'" /></div>
      </template>
      <template v-else-if="step < 6">
        <div class="payment-mode"><UIcon :name="mode === 'deducted' ? 'i-ph-minus-circle-bold' : 'i-ph-plus-circle-bold'" />{{ t(`knowledge.labels.${mode === 'deducted' ? 'feeDeducted' : 'feeSeparate'}`) }}</div>
        <div v-if="step === 4" class="payment-equation">
          <span><UIcon name="i-ph-receipt-bold" />{{ t('knowledge.labels.displayedAmount') }}<b>{{ amount }}</b></span>
          <span><UIcon name="i-ph-plus-bold" />{{ t('knowledge.labels.exchangeFee') }}<b>{{ fee }}</b></span>
          <span class="payment-equation__total"><UIcon name="i-ph-equals-bold" />{{ t('knowledge.labels.transferAmount') }}<b>{{ transfer }}</b></span>
        </div>
        <div v-else class="payment-equation">
          <span><UIcon name="i-ph-keyboard-bold" />{{ t('knowledge.labels.transferAmount') }}<b>{{ transfer }}</b></span>
          <span v-if="mode === 'deducted'"><UIcon name="i-ph-minus-bold" />{{ t('knowledge.labels.exchangeFee') }}<b>{{ fee }}</b></span>
          <span class="payment-equation__total"><UIcon name="i-ph-equals-bold" />{{ t('knowledge.labels.recipient') }}<b>{{ amount }}</b></span>
          <span v-if="mode === 'separate'" class="payment-equation__separate"><UIcon name="i-ph-gas-pump-bold" />{{ t('knowledge.labels.feeSeparate') }}<b>{{ fee }}</b></span>
        </div>
      </template>
      <template v-else-if="step < 8">
        <DemoFlow :nodes="flow" :phase="p" :active="step === 6 ? 0 : 1" :reduced-motion="reducedMotion" />
        <div class="payment-gas"><UIcon name="i-ph-gas-pump-bold" /><span>{{ t('knowledge.labels.networkFee') }}<small>{{ t('knowledge.labels.feeSeparate') }}</small></span><b>{{ step === 6 ? 'TRX' : 'ETH / TRX / SOL / APT' }}</b></div>
      </template>
      <template v-else-if="step < 10">
        <DemoFlow :nodes="confirmation" :phase="p" :active="step === 9 ? 2 : 1" :reduced-motion="reducedMotion" />
        <div class="payment-success" :class="{ 'is-ready': step === 9 || checked }"><UIcon :name="step === 9 || checked ? 'i-ph-check-circle-fill' : 'i-ph-hourglass-bold'" /><span>{{ t(`knowledge.labels.${step === 9 ? 'balanceReady' : checked ? 'confirmed' : 'pending'}`) }}</span></div>
      </template>
      <template v-else>
        <div class="payment-ticket"><span v-for="(key, index) in ['invoice', 'txid', 'network']" :key="key" :style="{ opacity: reducedMotion ? 1 : .25 + .75 * between(p, index * .16, index * .16 + .25), transform: `translateY(${reducedMotion ? 0 : 6 * (1 - between(p, index * .16, index * .16 + .25))}px)` }"><UIcon :name="['i-ph-receipt-bold', 'i-ph-hash-bold', 'i-ph-git-branch-bold'][index]" />{{ t(`knowledge.labels.${key}`) }}<UIcon name="i-ph-check-circle-fill" /></span></div>
        <div class="payment-private"><UIcon :name="step === 11 ? 'i-ph-lock-key-bold' : 'i-ph-chats-circle-bold'" />{{ t(`knowledge.labels.${step === 11 ? 'noSecrets' : 'bot'}`) }}</div>
      </template>
    </div>
    <template #footer><UIcon :name="checked ? 'i-ph-check-circle-fill' : 'i-ph-eye-bold'" /><span>{{ t('knowledge.labels.recipient') }}</span><strong>{{ amount }} {{ 'USDT' }}</strong></template>
  </DemoWindow>
</template>

<style scoped>
.payment-invoice { position: relative; display: grid; gap: .45rem; padding: .1rem 2rem .65rem 0; border-bottom: 1px solid var(--line); }
.payment-invoice > span { color: var(--text-faint); font-size: .65rem; }
.payment-invoice strong { color: var(--text); font: 600 clamp(1.05rem, 3.5vw, 1.45rem) var(--font-mono); }
.payment-invoice strong small { padding-left: .45rem; font-size: .65rem; color: var(--text-muted); }
.payment-invoice > .iconify { position: absolute; top: .5rem; right: 0; color: var(--accent); font-size: 1.8rem; opacity: .6; }
.payment-demo__content { display: grid; align-content: center; gap: .65rem; min-height: 145px; }
.payment-match { display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; gap: .7rem; color: var(--text-faint); }
.payment-match > span { display: flex; align-items: center; justify-content: center; gap: .45rem; padding: .6rem .45rem; background: var(--surface-soft); border-radius: .75rem; color: var(--text); font: .7rem var(--font-mono); }
.payment-match.is-checked > .iconify { color: var(--success); }
.payment-address { display: flex; align-items: center; gap: .75rem; color: var(--text-muted); font-size: .65rem; }
.payment-address > .iconify:first-child { color: var(--text); font-size: 2rem; }
.payment-address > .iconify:last-child { margin-left: auto; color: var(--accent); }
.payment-address span { display: grid; gap: .2rem; }
.payment-address code { color: var(--text); }
.payment-deadline { position: relative; display: flex; align-items: center; gap: .5rem; overflow: hidden; padding: .55rem .6rem .75rem; color: var(--text-muted); font-size: .65rem; }
.payment-deadline b { margin-left: auto; color: var(--text); font-family: var(--font-mono); }
.payment-deadline i { position: absolute; bottom: 0; inset-inline: 0; height: 3px; border-radius: 3px; background: var(--accent); transform-origin: left; }
.payment-mode { display: flex; align-items: center; gap: .4rem; color: var(--accent); font-size: .68rem; }
.payment-equation { display: grid; gap: .5rem; }
.payment-equation > span { display: grid; grid-template-columns: 1rem minmax(0, 1fr) auto; align-items: center; gap: .5rem; color: var(--text-muted); font-size: .65rem; }
.payment-equation b { font: 500 .86rem var(--font-mono); color: var(--text); }
.payment-equation__total { border-top: 1px solid var(--line); padding-top: .55rem; }
.payment-equation__total .iconify, .payment-equation__total b { color: var(--accent); }
.payment-equation__separate { margin-top: .2rem; padding-top: .5rem; border-top: 1px dashed var(--line); }
.payment-gas { display: flex; align-items: center; flex-wrap: wrap; gap: .55rem; padding: .65rem; border: 1px dashed var(--line); border-radius: .8rem; color: var(--text-muted); font-size: .65rem; }
.payment-gas > .iconify { font-size: 1.3rem; color: var(--accent); }
.payment-gas span { display: grid; gap: .2rem; }
.payment-gas small { color: var(--text-faint); font-size: .58rem; }
.payment-gas b { margin-left: auto; color: var(--text); font: .6rem var(--font-mono); }
.payment-success, .payment-private { display: flex; align-items: center; justify-content: center; gap: .55rem; color: var(--text-muted); font-size: .7rem; }
.payment-success > .iconify { font-size: 1.7rem; }
.payment-success.is-ready { color: var(--success); }
.payment-ticket { display: grid; gap: .5rem; }
.payment-ticket > span { display: flex; align-items: center; gap: .45rem; padding: .45rem; border-radius: .65rem; background: var(--surface-soft); color: var(--text-muted); font-size: .68rem; }
.payment-ticket > span > .iconify:last-child { margin-left: auto; color: var(--success); }
.payment-private { margin-top: .25rem; }
.knowledge-payment :deep(.demo-window__footer strong) { color: var(--accent); font: .65rem var(--font-mono); }
@media (max-width: 380px) { .payment-equation b { font-size: .72rem; } .payment-equation > span { gap: .3rem; font-size: .6rem; } .knowledge-payment :deep(.demo-window__footer) { flex-wrap: wrap; } }
</style>
