<script setup lang="ts">
import { defineAsyncComponent, shallowRef } from 'vue'
import { useI18n } from '@/i18n'

const RoutingDemo = defineAsyncComponent(() => import('./RoutingDemo.vue'))
const open = shallowRef(false)
const trigger = shallowRef<globalThis.HTMLElement | null>(null)
const { t } = useI18n()
function show(event: globalThis.MouseEvent): void {
  trigger.value = event.currentTarget instanceof globalThis.HTMLElement ? event.currentTarget : null
  open.value = true
}
</script>

<template>
  <UButton icon="i-ph-play-circle" color="neutral" variant="soft" size="sm" :label="t('diy.demo.open')" @click="show" />
  <RoutingDemo v-if="open" v-model:open="open" :return-focus="trigger" />
</template>
