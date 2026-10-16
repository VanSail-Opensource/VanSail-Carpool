<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { SubscriptionTemplateConfig } from '@/api/xnode'
import { useI18n } from '@/i18n'

const model = defineModel<SubscriptionTemplateConfig>({ required: true })
defineProps<{ busy: boolean }>()
const emit = defineEmits<{ save: [] }>()
const { t } = useI18n()
const user = shallowRef('')
const format = shallowRef('xray')
const selected = shallowRef('_default')
const page = shallowRef(0)
watch(format, () => { selected.value = '_default' })
const formats = computed(() => ['xray', 'sing-box', 'clash'].map(value => ({ value, label: t(`publicSubscription.formats.${value}`) })))
const options = computed(() => [{ value: '_default', label: t('nodeTemplates.useDefault') }, { value: '_builtin', label: t('nodeTemplates.builtin') }, ...model.value.templates.filter(item => item.format === format.value).map(item => ({ value: item.id, label: item.name }))])
const assignments = computed(() => Object.entries(model.value.assignments).flatMap(([user, formats]) => Object.entries(formats).map(([format, id]) => ({ user, format, id }))))
const visible = computed(() => assignments.value.slice(page.value * 20, (page.value + 1) * 20))
function assign() {
  const id = selected.value === '_builtin' ? '' : selected.value
  if (!user.value.trim() || selected.value !== '_default' && id && !model.value.templates.some(item => item.id === id && item.format === format.value)) return
  const refs = { ...model.value.assignments[user.value.trim()] }
  if (selected.value === '_default') delete refs[format.value]
  else refs[format.value] = id
  update(user.value.trim(), refs)
}
function update(user: string, refs: Record<string, string>) {
  const assignments = { ...model.value.assignments }
  if (Object.keys(refs).length) assignments[user] = refs
  else delete assignments[user]
  model.value = { ...model.value, assignments }
  emit('save')
}
function remove(user: string, format: string) { const refs = { ...model.value.assignments[user] }; delete refs[format]; update(user, refs) }
</script>

<template>
  <section class="grid gap-3 min-w-0">
    <h3>{{ t('nodeTemplates.assignments') }}</h3>
    <div class="grid gap-3 sm:grid-cols-3"><UFormField :label="t('nodeTemplates.userId')"><UInput v-model.trim="user" :disabled="busy" class="w-full" /></UFormField><UFormField :label="t('publicSubscription.format')"><USelect v-model="format" :items="formats" :disabled="busy" class="w-full" /></UFormField><UFormField :label="t('nodeTemplates.template')"><USelect v-model="selected" :items="options" :disabled="busy" class="w-full" /></UFormField></div>
    <UButton class="justify-self-end" :label="t('nodeTemplates.assign')" :disabled="!user || busy" @click="assign" />
    <p v-if="!assignments.length" class="text-muted">{{ t('nodeTemplates.noAssignments') }}</p>
    <div v-for="row in visible" :key="`${row.user}:${row.format}`" class="grid grid-cols-[minmax(0,1fr)_auto] gap-2 items-center text-sm"><div class="min-w-0"><code class="break-all">{{ row.user }}</code><p class="text-muted">{{ t(`publicSubscription.formats.${row.format}`) }} · {{ model.templates.find(item => item.id === row.id)?.name ?? t('nodeTemplates.builtin') }}</p></div><UButton variant="ghost" color="neutral" icon="i-ph-trash-bold" :aria-label="t('common.delete')" :disabled="busy" @click="remove(row.user, row.format)" /></div>
    <div v-if="assignments.length > 20" class="flex justify-end gap-2"><UButton variant="ghost" :label="t('nodeTemplates.previous')" :disabled="page === 0" @click="page--" /><UButton variant="ghost" :label="t('nodeTemplates.next')" :disabled="(page + 1) * 20 >= assignments.length" @click="page++" /></div>
  </section>
</template>
