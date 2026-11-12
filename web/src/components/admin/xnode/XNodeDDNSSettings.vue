<script setup lang="ts">
import { computed, reactive, toRaw, watch } from 'vue'
import type { DDNSHostBinding, XNodeEndpoint, XNodeSettings, XNodeGlobal } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeDDNSJob from './XNodeDDNSJob.vue'
import { builtinDDNSProvider, validDDNSSettings } from './ddnsForm'

const model = defineModel<XNodeSettings>({ required: true })
const props = withDefaults(defineProps<{ hosts?: readonly XNodeEndpoint[]; providers?: XNodeGlobal['providers']; providersLoading?: boolean }>(), { hosts: () => [], providers: () => ({}), providersLoading: false })
const emit = defineEmits<{ valid: [value: boolean] }>()
const { t } = useI18n()
const jobs = computed(() => model.value.ddns ?? [])
type Job = NonNullable<XNodeSettings['ddns']>[number]
const rowKeys = new WeakMap<Job, number>()
const pendingBindings = reactive(new Map<number, DDNSHostBinding>())
let nextRowKey = 0
function rowKey(job: Job): number {
  const raw = toRaw(job)
  if (!rowKeys.has(raw)) rowKeys.set(raw, nextRowKey++)
  return rowKeys.get(raw)!
}
function bindingFor(job: Job): DDNSHostBinding { return pendingBindings.get(rowKey(job)) ?? model.value.ddnsBindings?.[job.id] ?? {} }
function setJob(index: number, job: Job): void {
  const previous = jobs.value[index]
  if (previous) rowKeys.set(toRaw(job), rowKey(previous))
  model.value = { ...model.value, ddns: jobs.value.map((value, position) => position === index ? job : value) }
}
const providers = computed(() => {
  const values = [{ value: builtinDDNSProvider, label: t('nodeForms.builtinDDNS') }, ...Object.keys(props.providers).sort().map(value => ({ value, label: value }))]
  for (const job of jobs.value) if (job.provider && !values.some(value => value.value === job.provider)) values.push({ value: job.provider, label: job.provider })
  return values
})
function add(): void {
  let ordinal = 1
  while (jobs.value.some(job => job.id === `ddns-${ordinal}`)) ordinal++
  model.value = { ...model.value, ddns: [...jobs.value, { id: `ddns-${ordinal}`, provider: '', zone: '', name: '', ttl_seconds: 300, interval_seconds: 300 }] }
}
const valid = computed(() => validDDNSSettings(model.value, props.hosts))
watch(valid, value => emit('valid', value), { immediate: true })
const used = computed(() => jobs.value.flatMap(job => { const binding = bindingFor(job); return [...(binding.ipv4HostIds ?? []), ...(binding.ipv6HostIds ?? [])] }))
function setBinding(job: Job, binding: DDNSHostBinding): void {
  if (jobs.value.filter(value => value.id === job.id).length > 1) pendingBindings.set(rowKey(job), binding)
  else model.value = { ...model.value, ddnsBindings: { ...model.value.ddnsBindings, [job.id]: binding } }
}
watch(() => jobs.value.map(job => ({ key: rowKey(job), id: job.id })), (rows, previous) => {
  const original = model.value.ddnsBindings ?? {}
  const bindings = { ...original }, ids = rows.map(row => row.id)
  let changed = false
  for (const row of rows) {
    const old = previous.find(value => value.key === row.key)
    if (old && old.id !== row.id && !pendingBindings.has(row.key)) pendingBindings.set(row.key, original[old.id] ?? {})
    const pending = pendingBindings.get(row.key)
    if (pending && ids.filter(id => id === row.id).length === 1) {
      bindings[row.id] = pending; pendingBindings.delete(row.key); changed = true
    }
  }
  for (const key of pendingBindings.keys()) if (!rows.some(row => row.key === key)) pendingBindings.delete(key)
  for (const id of Object.keys(bindings)) if (!ids.includes(id)) { delete bindings[id]; changed = true }
  if (changed) model.value = { ...model.value, ddnsBindings: bindings }
})
function remove(index: number): void { model.value = { ...model.value, ddns: jobs.value.filter((_, current) => current !== index) } }
</script>

<template>
  <div class="grid gap-4 py-4">
    <XNodeDDNSJob v-for="(job, index) in jobs" :key="rowKey(job)" :model-value="job" :binding="bindingFor(job)" :providers="providers" :loading="providersLoading" :hosts="hosts" :used="used" @update:model-value="setJob(index, $event)" @update:binding="setBinding(job, $event)" @remove="remove(index)" />
    <UButton variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('xnode.addDDNS')" class="justify-self-start" @click="add" />
  </div>
</template>
