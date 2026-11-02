<script setup lang="ts">
import { squadFeatureIcon } from '@/components/squad-profile/featureIcons'

defineProps<{ features: readonly { icon: string; title: string; detail?: string }[] }>()
</script>

<template>
  <section v-if="features.length" class="squad-features" :aria-label="$t('catalog.features')">
    <div class="squad-features__heading"><span>{{ $t('catalog.features') }}</span></div>
    <ul>
      <li v-for="(feature, index) in features" :key="index">
        <span class="squad-features__icon"><UIcon :name="squadFeatureIcon(feature.icon)" aria-hidden="true" /></span>
        <span class="squad-features__copy"><strong>{{ feature.title }}</strong><small v-if="feature.detail">{{ feature.detail }}</small></span>
      </li>
    </ul>
  </section>
</template>

<style scoped>
.squad-features { min-width: 0; padding-block: .5rem; border-top: 1px solid var(--line); }
.squad-features__heading { margin-bottom: .45rem; color: var(--text-faint); font: 650 .625rem var(--font-mono); }
.squad-features ul { display: grid; gap: .4rem; margin: 0; padding: 0; list-style: none; }
.squad-features li { display: grid; grid-template-columns: 1.125rem minmax(0, 1fr); align-items: start; gap: .4rem; min-width: 0; }
.squad-features__icon { display: grid; place-items: center; width: 1.125rem; height: 1.125rem; color: var(--squad-profile-tone, var(--accent)); background: color-mix(in srgb, var(--squad-profile-tone, var(--accent)) 11%, var(--surface)); border-radius: 5px; }
.squad-features__copy { display: block; min-width: 0; padding-top: .05rem; color: var(--text-muted); font-size: .69rem; line-height: 1.4; overflow-wrap: anywhere; text-align: left; }
.squad-features__copy strong { display: block; color: var(--text); font-weight: 650; }
.squad-features__copy small { display: block; font-size: inherit; color: var(--text-faint); }
</style>
