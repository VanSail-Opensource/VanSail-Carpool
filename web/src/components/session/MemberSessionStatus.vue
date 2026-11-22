<script setup lang="ts">
defineProps<{ authenticated: boolean }>()
</script>

<template>
  <div class="page">
    <p class="member-session-status" role="status" aria-atomic="true">
      {{ $t(authenticated ? 'auth.loadingPage' : 'auth.loadingAccount') }}
    </p>
  </div>
</template>

<style scoped>
.member-session-status {
  color: var(--text-muted);
  font-size: 0.875rem;
}
</style>
