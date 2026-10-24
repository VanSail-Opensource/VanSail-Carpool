<script setup lang="ts">
import { shallowRef } from 'vue'
import CatalogExperience from '@/components/catalog/CatalogExperience.vue'
import AdminSquadEditor from '@/components/admin/AdminSquadEditor.vue'
import type { SquadProduct, SquadProductWrite } from '@/api/types'

defineProps<{ admin: boolean; squad: SquadProduct }>()
const saved = shallowRef<SquadProductWrite | null>(null)
</script>
<template>
  <UApp><main class="payg-audit"><AdminSquadEditor v-if="admin" :squad="squad" :busy="false" @save="saved = $event" /><CatalogExperience v-else /><output v-if="saved" data-saved>{{ JSON.stringify(saved) }}</output></main></UApp>
</template>
<style scoped>
.payg-audit { max-width: 1150px; margin: auto; min-width: 0; padding: 12px; }
</style>
