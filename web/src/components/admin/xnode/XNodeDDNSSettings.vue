<script setup lang="ts">
import { computed, watch } from 'vue'
import type { DDNSHostBinding, XNodeEndpoint, XNodeSettings } from '@/api/xnode'
import { useI18n } from '@/i18n'
import XNodeDDNSJob from './XNodeDDNSJob.vue'
import { useDDNSDiscovery } from './useDDNSDiscovery'
import { builtinDDNSProvider, validDDNSSettings } from './ddnsForm'

const model = defineModel<XNodeSettings>({ required: true })
const props = withDefaults(defineProps<{ hosts?: readonly XNodeEndpoint[] }>(), { hosts: () => [] })
const emit = defineEmits<{ valid: [value: boolean] }>()
const { t } = useI18n()
const jobs = computed(() => model.value.ddns ?? [])
const discovery = useDDNSDiscovery()
const providers = computed(() => {
  const values = [{ value: builtinDDNSProvider, label: t('nodeForms.builtinDDNS') }, ...discovery.items.value]
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
const used = computed(() => Object.values(model.value.ddnsBindings ?? {}).flatMap(binding => [...(binding.ipv4HostIds ?? []), ...(binding.ipv6HostIds ?? [])]))
function setBinding(id: string, binding: DDNSHostBinding): void { model.value = { ...model.value, ddnsBindings: { ...model.value.ddnsBindings, [id]: binding } } }
watch(() => jobs.value.map(job => job.id), (ids, previous) => {
  const bindings = { ...model.value.ddnsBindings }
  let changed = false
  for (let index = 0; index < ids.length; index++) {
    const old = previous[index], next = ids[index]!
    if (old !== undefined && old !== next && bindings[old] && !ids.includes(old) && !bindings[next]) {
      bindings[next] = bindings[old]; delete bindings[old]; changed = true
    }
  }
  for (const id of Object.keys(bindings)) if (!ids.includes(id)) { delete bindings[id]; changed = true }
  if (changed) model.value = { ...model.value, ddnsBindings: bindings }
})
function remove(index: number): void { model.value = { ...model.value, ddns: jobs.value.filter((_, current) => current !== index) } }
</script>

<template>
  <div class="grid gap-4 py-4">
    <div v-if="discovery.error.value" role="alert"><p>{{ discovery.error.value }}</p><UButton :label="t('common.tryAgain')" variant="link" @click="discovery.load" /></div>
    <XNodeDDNSJob v-for="(job, index) in jobs" :key="job.id" v-model="jobs[index]!" :binding="model.ddnsBindings?.[job.id] ?? {}" :providers="providers" :loading="discovery.loading.value" :hosts="hosts" :used="used" @update:binding="setBinding(job.id, $event)" @remove="remove(index)" />
    <UButton variant="outline" color="neutral" icon="i-ph-plus-bold" :label="t('xnode.addDDNS')" class="justify-self-start" @click="add" />
  </div>
</template>
