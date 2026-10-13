<script setup lang="ts">
import { selectionHaptic } from '@/utils/feedback'

withDefaults(defineProps<{
  id: string
  label: string
  help?: string
  disabled?: boolean
}>(), {
  help: '',
  disabled: false,
})

const model = defineModel<boolean>({ required: true })
</script>

<template>
  <label :for="id" class="switch-field" :class="{ 'switch-field--disabled': disabled }">
    <span class="switch-field__copy">
      <span :id="`${id}-label`" class="switch-field__label">{{ label }}</span>
      <small v-if="help" :id="`${id}-help`">{{ help }}</small>
    </span>
    <USwitch
      :id="id"
      v-model="model"
      class="switch-control"
      :disabled="disabled"
      :aria-labelledby="`${id}-label`"
      :aria-describedby="help ? `${id}-help` : undefined"
      @update:model-value="selectionHaptic"
    />
  </label>
</template>

<style scoped>
.switch-field { min-height: 52px; display: flex; flex-direction: row; align-items: center; justify-content: space-between; gap: 1rem; cursor: pointer; }
.switch-field--disabled { cursor: not-allowed; }
.switch-field__copy { min-width: 0; overflow-wrap: anywhere; }
.switch-field__label, .switch-field__copy small { display: block; }
.switch-field__label { color: var(--text-muted); font-size: 0.78rem; font-weight: 700; }
.switch-field__copy small { margin-top: 0.25rem; color: var(--text-faint); font-size: 0.68rem; line-height: 1.4; }
.switch-control { flex: 0 0 auto; }
</style>
