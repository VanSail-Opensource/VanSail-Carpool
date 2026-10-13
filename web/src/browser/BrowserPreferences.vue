<script setup lang="ts">
import { computed } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import { useI18n } from '@/i18n'
import { useBrowserTheme, type BrowserTheme } from './theme'

const { locale, setLocale, t } = useI18n()
const theme = useBrowserTheme()
const themeIcons = { system: 'i-ph-monitor-bold', light: 'i-ph-sun-bold', dark: 'i-ph-moon-bold' } as const
const themeItems = computed(() => (['system', 'light', 'dark'] as BrowserTheme[]).map(value => ({
  label: t(`browser.theme.${value}`),
  icon: themeIcons[value],
  onSelect: () => theme.setTheme(value),
})))
function selectLocale(value: string): void { if (value === 'en' || value === 'zh-CN') setLocale(value) }
</script>

<template>
  <div class="browser-preferences">
    <ULocaleSelect :model-value="locale" :locales="[en, zh_cn]" :search-input="false" variant="ghost" color="neutral" size="sm" :aria-label="t('app.language')" @update:model-value="selectLocale" />
    <UDropdownMenu :items="themeItems">
      <UButton :icon="theme.dark.value ? 'i-ph-moon-bold' : 'i-ph-sun-bold'" color="neutral" variant="ghost" size="sm" :aria-label="t('browser.theme.label')" />
    </UDropdownMenu>
  </div>
</template>
