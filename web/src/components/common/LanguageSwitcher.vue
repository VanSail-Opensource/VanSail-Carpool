<script setup lang="ts">
import { computed } from 'vue'

import { useI18n } from '@/i18n'
import type { Locale } from '@/i18n/generated'
import { selectionHaptic } from '@/utils/feedback'

const { locale, locales, setLocale, t } = useI18n()
const items = computed(() => locales.value.map((value) => ({
  value,
  label: value === 'en' ? t('app.english') : t('app.simplifiedChinese'),
})))

function selectLocale(value: Locale): void {
  if (value === locale.value) return
  setLocale(value)
  selectionHaptic()
}
</script>

<template>
  <USelect
    class="language-switcher"
    :model-value="locale"
    :items="items"
    value-key="value"
    :aria-label="t('app.language')"
    @update:model-value="selectLocale($event as Locale)"
  />
</template>
