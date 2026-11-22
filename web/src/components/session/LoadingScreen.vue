<script setup lang="ts">
import { usePendingStatus } from '@/composables/usePendingStatus'
import '@/styles/member-startup.css'

const { slow } = usePendingStatus(() => true)
</script>

<template>
  <main class="session-loading">
    <div class="session-loading__content">
      <div class="session-loading__signal" aria-hidden="true"></div>
      <h1 class="session-loading__heading">{{ $t('app.name') }}</h1>
      <p class="session-loading__status" role="status" aria-atomic="true">
        {{ $t(slow ? 'auth.loadingAccountSlow' : 'auth.loadingAccount') }}
      </p>
    </div>
  </main>
</template>
