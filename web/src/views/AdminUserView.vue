<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AdminUserProfilePage from '@/components/admin/users/AdminUserProfilePage.vue'

const route = useRoute()
const router = useRouter()
const userId = computed(() => String(route.params.userId ?? ''))

function backToUsers(): void {
  void router.push('/admin/users')
}

</script>

<template>
  <AdminUserProfilePage :user-id="userId" @back="backToUsers" />
</template>
