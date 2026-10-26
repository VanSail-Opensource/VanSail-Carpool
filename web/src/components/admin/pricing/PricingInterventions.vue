<script setup lang="ts">
import { shallowRef } from 'vue'
import { pricingAnalysisApi, type PricingRecord } from '@/api/pricingAnalysis'
import { useI18n } from '@/i18n'
import { formatDate } from '@/utils/format'
import InlineNotice from '@/components/common/InlineNotice.vue'
defineProps<{ items: PricingRecord[] }>()
const emit = defineEmits<{ changed: [] }>()
const { t } = useI18n()
const reason = shallowRef('')
const busy = shallowRef(false)
const error = shallowRef('')
async function action(item: PricingRecord): Promise<void> {
  if (busy.value || !reason.value.trim()) return
  busy.value = true
  error.value = ''
  try {
    await pricingAnalysisApi.cancel(String(item.id), reason.value.trim())
    reason.value = ''
    emit('changed')
  } catch { error.value = t('pricing.saveError') }
  finally { busy.value = false }
}
</script>
<template>
  <section class="pricing-interventions">
    <h3>{{ t('pricing.interventions') }}</h3>
    <p v-if="!items.length">{{ t('pricing.emptyRecords') }}</p>
    <template v-else>
      <UFormField :label="t('pricing.reason')"><UInput v-model="reason" :maxlength="1000" class="w-full" /></UFormField>
      <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      <ol>
        <li v-for="item in items" :key="String(item.id)">
          <div><strong>{{ t(`pricing.commands.${item.command}`) }}</strong><span>{{ t(`pricing.states.${item.status}`) }} · {{ item.scope_id }}</span><small>{{ formatDate(String(item.created_at)) }} · {{ item.created_by }} · {{ item.reason }}</small></div>
          <div class="row-actions">
            <UButton v-if="item.status === 'ACTIVE'" color="neutral" variant="ghost" :label="t('common.cancel')" :disabled="busy || !reason.trim()" @click="action(item)" />
          </div>
        </li>
      </ol>
    </template>
  </section>
</template>
<style scoped>
.pricing-interventions { display: grid; gap: 1rem; min-width: 0; }.pricing-interventions h3 { margin: 0; font-size: 1rem; }
.pricing-interventions ol { list-style: none; padding: 0; margin: 0; max-height: 30rem; overflow: auto; }.pricing-interventions li { display: flex; flex-wrap: wrap; justify-content: space-between; gap: .75rem; padding: 1rem 0; border-top: 1px solid var(--line); }
.pricing-interventions strong, .pricing-interventions span, .pricing-interventions small { display: block; overflow-wrap: anywhere; }.pricing-interventions small, .pricing-interventions span { margin-top: .25rem; color: var(--text-muted); font-size: .8rem; }
</style>
