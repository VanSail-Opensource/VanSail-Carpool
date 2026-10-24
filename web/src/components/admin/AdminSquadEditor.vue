<script setup lang="ts">
import { reactive, shallowRef, watch } from 'vue'
import { motion } from 'motion-v'

import type { SquadProduct, SquadProductWrite, SquadProfileWrite } from '@/api/types'
import MarkdownEditorField from '@/components/common/MarkdownEditorField.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SwitchField from '@/components/common/SwitchField.vue'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'
import AdminSquadProfileEditor from './squad-profile/AdminSquadProfileEditor.vue'
import { editableProfile } from './squad-profile/profileForm'

const props = defineProps<{
  squad: SquadProduct
  busy: boolean
  error?: string | null
}>()

const emit = defineEmits<{
  cancel: []
  save: [payload: SquadProductWrite]
}>()

const draft = reactive({
  billingMode: 'combo' as 'combo' | 'payg',
  paygPrice: '',
  description: '',
  priceTxb: '',
  visible: false,
  stockLimit: '',
  activationRequired: false,
  activationCode: '',
  profile: null as SquadProfileWrite | null,
})
const profileAttempted = shallowRef(false)
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()

watch(() => props.squad, (squad) => {
  Object.assign(draft, {
    billingMode: squad.billingMode ?? 'combo',
    paygPrice: txbInputFromMinor(squad.paygPrice?.minor ?? '0'),
    description: squad.description,
    priceTxb: txbInputFromMinor(squad.price.minor),
    visible: squad.visible,
    stockLimit: squad.stockLimit === null ? '' : String(squad.stockLimit ?? ''),
    activationRequired: squad.activationRequired,
    activationCode: '',
    profile: editableProfile(squad.profile),
  })
  profileAttempted.value = false
}, { immediate: true })

function submit(): void {
  const priceTxbMinor = moneyFromTxbInput(draft.priceTxb)
  const paygPriceTxbMinor = moneyFromTxbInput(draft.paygPrice)
  if (draft.billingMode === 'payg' && (!paygPriceTxbMinor || BigInt(paygPriceTxbMinor) <= 0n)) return
  const stockLimitStr = String(draft.stockLimit ?? '').trim()
  const stockLimit = stockLimitStr === '' ? null : Number(stockLimitStr)
  if (!priceTxbMinor || draft.description.length > 1000 || (stockLimit !== null && (!Number.isInteger(stockLimit) || stockLimit < 0)) || !draft.profile) {
    profileAttempted.value = true
    return
  }
  emit('save', {
    billingMode: draft.billingMode,
    paygPriceTxbMinor: paygPriceTxbMinor ?? '0',
    groupId: props.squad.groupId,
    name: props.squad.name,
    description: draft.description.trim(),
    priceTxbMinor,
    visible: draft.visible,
    stockLimit,
    activationRequired: draft.activationRequired,
    ...(draft.activationCode.trim() ? { activationCode: draft.activationCode.trim() } : {}),
    profile: draft.profile,
  })
}
</script>

<template>
  <USlideover :open="true" :title="t('adminSquad.edit', { name: squad.name })" :description="t('adminSquad.identityHint')" :dismissible="!busy" @update:open="!$event && emit('cancel')">
    <template #body>
      <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      <UForm :state="draft" class="admin-editor-form" @submit="submit">
        <AdminSquadProfileEditor v-model:profile="draft.profile" :description="draft.description" :show-validation="profileAttempted" />
        <MarkdownEditorField v-model="draft.description" class="catalog-editor__wide" :label="t('adminSquad.description')" :placeholder="t('adminSquad.descriptionPlaceholder')" :maxlength="1000" />
        <UFormField name="billingMode" :label="t('payg.billingMode')">
          <USelect v-model="draft.billingMode" :items="[{ label: t('payg.combo'), value: 'combo' }, { label: t('payg.title'), value: 'payg' }]" />
        </UFormField>
        <InlineNotice v-if="draft.billingMode === 'payg'">{{ t('payg.adminHint') }}</InlineNotice>
        <TxbAmountField v-if="draft.billingMode === 'payg'" id="squad-payg-price" v-model="draft.paygPrice" :label="t('payg.price')" min-minor="1" required />
        <TxbAmountField v-else id="squad-price" v-model="draft.priceTxb" :label="t('adminSquad.price')" min-minor="0" required />
        <UFormField v-if="draft.billingMode === 'combo'" name="squad-stock-limit" :label="t('adminSquad.stockLimit')" :hint="t('adminSquad.stockLimitHint')"><UInput v-model="draft.stockLimit" type="number" min="0" step="1" inputmode="numeric" :placeholder="t('adminSquad.stockUnlimited')" /></UFormField>
        <SwitchField v-if="draft.billingMode === 'combo'" id="squad-visible" v-model="draft.visible" :label="t('adminSquad.visible')" :help="t('adminSquad.visibleHint')" />
        <SwitchField v-if="draft.billingMode === 'combo'" id="squad-activation-required" v-model="draft.activationRequired" :label="t('adminSquad.activationRequired')" :help="t('adminSquad.activationRequiredHint')" />
        <motion.div v-if="draft.billingMode === 'combo' && draft.activationRequired" layout :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 4 }" :animate="{ opacity: 1, y: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }"><UFormField name="squad-activation-code" :label="t('adminSquad.activationCode')" :hint="t('adminSquad.activationCodeHint')" required><UInput v-model="draft.activationCode" type="password" autocomplete="new-password" inputmode="text" /></UFormField></motion.div>
        <UButton class="catalog-editor__wide" type="submit" icon="i-ph-floppy-disk" :loading="busy" :disabled="busy" :label="busy ? t('common.saving') : t('adminSquad.saveSquad')" />
      </UForm>
    </template>
  </USlideover>
</template>

<style scoped>
</style>
