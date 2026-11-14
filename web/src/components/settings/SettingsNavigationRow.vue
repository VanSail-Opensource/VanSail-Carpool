<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const props = defineProps<{ label: string; icon: string; path: string; compact?: boolean }>()
const route = useRoute()
const router = useRouter()
const active = computed(() => route.path === props.path)
function navigate(): void { void router.push(props.path).catch(() => undefined) }
</script>

<template>
  <UButton type="button" color="neutral" variant="ghost" class="settings-navigation-row" :class="{ 'settings-navigation-row--compact': compact, 'is-active': active }" :aria-current="active ? 'page' : undefined" data-haptic="navigate" @click="navigate">
    <span class="settings-navigation-row__symbol"><UIcon :name="icon" aria-hidden="true" /></span>
    <strong>{{ label }}</strong>
    <UIcon name="i-ph-arrow-up-right-bold" class="settings-navigation-row__arrow" aria-hidden="true" />
  </UButton>
</template>

<style scoped>
.settings-navigation-row { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: .75rem; width: 100%; min-height: 62px; padding: .65rem .85rem; color: var(--text); border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface); text-align: left; transition: border-color 180ms ease, background 180ms ease; }
.settings-navigation-row strong { min-width: 0; font-size: .875rem; font-weight: 600; white-space: normal; overflow-wrap: anywhere; }
.settings-navigation-row__symbol { display: grid; place-items: center; width: 2.1rem; height: 2.1rem; border-radius: .65rem; color: var(--accent); background: var(--accent-soft); font-size: 1.1rem; }
.settings-navigation-row__arrow { color: var(--text-faint); font-size: .9rem; transition: transform 180ms ease; }
.settings-navigation-row:hover, .settings-navigation-row.is-active { border-color: var(--accent-line); background: var(--surface-raised); }
.settings-navigation-row:hover .settings-navigation-row__arrow { transform: translate(2px, -2px); color: var(--accent); }
.settings-navigation-row.is-active { box-shadow: inset 2px 0 0 var(--accent); }
.settings-navigation-row--compact { min-height: 48px; gap: .5rem; padding: .5rem; border-radius: var(--radius-control); }
.settings-navigation-row--compact strong { font-size: .75rem; }
.settings-navigation-row--compact .settings-navigation-row__symbol { width: 1.75rem; height: 1.75rem; font-size: 1rem; }
@media (prefers-reduced-motion: reduce) { .settings-navigation-row, .settings-navigation-row__arrow { transition: none; } }
</style>
