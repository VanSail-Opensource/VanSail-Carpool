<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { DIYField, DIYOverrides } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import DIYSettingsGroup from './DIYSettingsGroup.vue'
import { diyFieldActive } from './diyApplicability'

const model = defineModel<DIYOverrides>({ required: true })
const props = defineProps<{ fields: DIYField[]; groups: string[]; errors: Record<string, string>; disabled?: boolean }>()
const { t } = useI18n()
const advanced = shallowRef(false)
const fields = computed(() => props.fields.filter(field => props.groups.includes(field.group) && diyFieldActive(field, model.value)))
const basic = computed(() => fields.value.filter(field => !field.advanced))
const groups = computed(() => props.groups.filter(group => fields.value.some(field => field.group === group && field.advanced)))
</script>

<template>
  <div class="grid gap-4 min-w-0">
    <p v-if="!fields.length" class="text-sm text-muted">{{ t('diy.noApplicableFields') }}</p>
    <DIYSettingsGroup v-if="basic.length" v-model="model" :fields="basic" :errors="errors" :disabled="disabled" />
    <details v-if="groups.length" @toggle="advanced = ($event.target as globalThis.HTMLDetailsElement).open">
      <summary class="cursor-pointer py-2 text-sm">{{ t('diy.advanced') }}</summary>
      <div v-if="advanced" class="grid gap-5 py-3">
        <DIYSettingsGroup v-for="group in groups" :key="group" v-model="model" :group="group" :fields="fields.filter(field => field.group === group && field.advanced)" :errors="errors" :disabled="disabled" />
      </div>
    </details>
  </div>
</template>
