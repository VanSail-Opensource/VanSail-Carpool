<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from '@/i18n'
import { haptic } from '@/utils/telegram'
import { mobileNavigationItems } from './navigation'
import { usePreferencesStore } from '@/stores/preferences'

const props = defineProps<{ isAdmin: boolean, isNewUser: boolean }>()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const preferences = usePreferencesStore()
const items = computed(() => mobileNavigationItems(props.isAdmin, preferences.showActivity, props.isNewUser).map(item => ({
  label: t(item.labelKey), icon: item.icon, value: item.to,
})))
const active = computed(() => {
  if (items.value.some(item => item.value === route.path)) return route.path
  return route.path.startsWith('/admin') && props.isAdmin ? '/admin/settings' : undefined
})

function navigate(value: string | number): void {
  haptic('navigate')
  void router.push(String(value)).catch(() => undefined)
}
</script>

<template>
  <nav class="bottom-nav" :aria-label="t('nav.primary')">
    <UTabs
      :model-value="active"
      :items="items"
      :content="false"
      size="sm"
      class="bottom-nav__tabs"
      :ui="{
        list: 'bottom-nav__list',
        indicator: 'bottom-nav__indicator',
        trigger: 'bottom-nav__item',
        leadingIcon: 'bottom-nav__icon',
        label: 'bottom-nav__label',
      }"
      @update:model-value="navigate"
    />
  </nav>
</template>
