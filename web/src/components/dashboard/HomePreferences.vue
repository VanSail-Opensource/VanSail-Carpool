<script setup lang="ts">
import CurrencyControl from '@/components/layout/CurrencyControl.vue'
import LanguageControl from '@/components/layout/LanguageControl.vue'
</script>

<template>
  <div class="home-preferences" role="group" :aria-label="$t('home.preferences.label')">
    <LanguageControl />
    <span class="home-preferences__divider" aria-hidden="true">·</span>
    <CurrencyControl />
  </div>
</template>

<style scoped>
.home-preferences {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 0.25rem 0.55rem;
  padding-block: 0.25rem 0.5rem;
  animation: home-preferences-in 260ms var(--ease-out) both;
}

.home-preferences__divider {
  color: var(--text-faint);
  font-size: 0.9rem;
  line-height: 1;
}

.home-preferences :deep(.language-control),
.home-preferences :deep(.currency-control) {
  min-height: 44px;
}

@keyframes home-preferences-in {
  from {
    opacity: 0;
    transform: translateY(4px);
  }

  to {
    opacity: 1;
    transform: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .home-preferences {
    animation: none;
  }
}
</style>
