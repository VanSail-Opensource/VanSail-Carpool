<script setup lang="ts">
import { useRouter } from 'vue-router'

const props = defineProps<{ label: string; icon: string; path: string }>()
const router = useRouter()
function navigate(): void { void router.push(props.path).catch(() => undefined) }
</script>

<template>
  <UButton type="button" color="neutral" variant="ghost" class="settings-navigation-row" data-haptic="navigate" @click="navigate">
    <UIcon :name="icon" class="settings-navigation-row__icon" aria-hidden="true" />
    <strong>{{ label }}</strong>
    <UIcon name="i-ph-caret-right" class="settings-navigation-row__icon" aria-hidden="true" />
  </UButton>
</template>

<style scoped>
.settings-navigation-row { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: .75rem; width: 100%; min-height: 60px; padding: .75rem 0; color: var(--text); border: 0; background: transparent; text-align: left; }
.settings-navigation-row strong { font-size: .875rem; font-weight: 600; white-space: normal; }
.settings-navigation-row__icon { color: var(--text-muted); font-size: 1.1rem; }
</style>
