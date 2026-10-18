<script setup lang="ts">
import { computed, reactive, shallowRef, watch } from 'vue'
import { api } from '@/api/client'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n, localizedError } from '@/i18n'
import { moneyFromTxbInput } from '@/utils/format'
const props = defineProps<{ userId: string }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const open = shallowRef(false)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const form = reactive({ amountTxb: '', reason: '' })
const valid = computed(() => { const amount = moneyFromTxbInput(form.amountTxb); return amount !== null && globalThis.BigInt(amount) > 0n && form.reason.length >= 4 })
let generation = 0
watch(() => props.userId, () => { generation++; open.value = false; error.value = null; form.amountTxb = ''; form.reason = '' })
async function adjust(sign: 1 | -1): Promise<void> {
  const amount = moneyFromTxbInput(form.amountTxb)
  if (!amount || !valid.value || busy.value) return
  const ticket = generation
  busy.value = true; error.value = null
  try {
    await api.adjustBalance(props.userId, sign === -1 ? `-${amount}` : amount, form.reason)
    if (ticket === generation) { open.value = false; form.amountTxb = ''; form.reason = ''; emit('changed') }
  } catch (caught) { if (ticket === generation) error.value = localizedError(caught, 'errors.adminAction') }
  finally { busy.value = false }
}
</script>
<template>
  <div><UButton :label="t('adminUsers.adjust')" icon="i-ph-coins" color="neutral" variant="outline" @click="open = true" /></div>
  <USlideover v-model:open="open" :title="t('adminUsers.adjust')" :description="t('adminUsers.adjustHint')" :dismissible="!busy">
    <template #body><div class="grid gap-4"><InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice><TxbAmountField id="profile-balance-adjustment" v-model="form.amountTxb" :label="t('adminUsers.amount')" min-minor="1" required /><UFormField :label="t('adminReason.reason')" required><UInput v-model.trim="form.reason" :maxlength="300" :disabled="busy" /></UFormField><div class="row-actions"><UButton :label="t('adminUsers.add')" icon="i-ph-plus" :disabled="!valid || busy" :loading="busy" @click="adjust(1)" /><UButton :label="t('adminUsers.deduct')" icon="i-ph-minus" color="error" variant="outline" :disabled="!valid || busy" @click="adjust(-1)" /></div></div></template>
  </USlideover>
</template>
