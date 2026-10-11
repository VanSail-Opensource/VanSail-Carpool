<script setup lang="ts">
import SwitchField from '@/components/common/SwitchField.vue'
import type { IPLookupRetryPolicy } from '@/api/ipLookup'

const retry = defineModel<IPLookupRetryPolicy>({ required: true })
defineProps<{ disabled?: boolean }>()
</script>

<template>
  <section class="retry-settings">
    <h2>{{ $t('ipLookup.retryHeading') }}</h2>
    <p>{{ $t('ipLookup.retryHelp') }}</p>
    <SwitchField id="ip-lookup-retry-enabled" v-model="retry.enabled" :label="$t('ipLookup.retryEnable')" :help="$t('ipLookup.retryEnableHelp')" :disabled="disabled" />
    <div class="retry-fields">
      <UFormField name="retry-max-attempts" :label="$t('ipLookup.retryMaxAttempts')">
        <UInputNumber v-model="retry.maxAttempts" class="w-full" :min="1" :max="10" :step="1" :disabled="disabled || !retry.enabled" />
      </UFormField>
      <UFormField name="retry-delay-seconds" :label="$t('ipLookup.retryDelaySeconds')" :hint="$t('ipLookup.retryDelayHint')">
        <UInputNumber v-model="retry.delaySeconds" class="w-full" :min="1" :max="86400" :step="1" :disabled="disabled || !retry.enabled" />
      </UFormField>
    </div>
  </section>
</template>

<style scoped>
.retry-settings { display: grid; gap: 0.8rem; }
.retry-settings h2 { font-size: 1rem; margin: 0; }
.retry-settings p { color: var(--text-muted); font-size: 0.82rem; margin: 0; }
.retry-fields { display: grid; gap: 1rem; }
@media (min-width: 900px) { .retry-fields { grid-template-columns: repeat(2, minmax(0, 20rem)); } }
</style>
