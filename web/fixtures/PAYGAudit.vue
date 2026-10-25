<script setup lang="ts">
import { shallowRef } from 'vue'
import CatalogExperience from '@/components/catalog/CatalogExperience.vue'
import AdminSquadEditor from '@/components/admin/AdminSquadEditor.vue'
import SquadPricingCard from '@/components/catalog/SquadPricingCard.vue'
import type { SquadProduct, SquadProductWrite } from '@/api/types'

defineProps<{ admin: boolean; squad: SquadProduct; cards: boolean; squads: SquadProduct[] }>()
const saved = shallowRef<SquadProductWrite | null>(null)
const entered = shallowRef(!new globalThis.URLSearchParams(globalThis.location.search).has('deferred'))
</script>
<template>
  <UApp><main class="payg-audit"><AdminSquadEditor v-if="admin" :squad="squad" :busy="false" @save="saved = $event" /><div v-else-if="cards" class="payg-audit__cards"><SquadPricingCard v-for="item in squads" :key="item.id" :squad="item" :selected="false" :included="false" :featured="false" /></div><CatalogExperience v-else-if="entered" /><UButton v-else :label="$t('nav.explore')" @click="entered = true" /><output v-if="saved" data-saved>{{ JSON.stringify(saved) }}</output></main></UApp>
</template>
<style scoped>
.payg-audit { max-width: 1150px; margin: auto; min-width: 0; padding: 12px; }
.payg-audit__cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 19rem), 1fr)); gap: 12px; }
.payg-audit output { display: block; white-space: pre-wrap; overflow-wrap: anywhere; }
</style>
