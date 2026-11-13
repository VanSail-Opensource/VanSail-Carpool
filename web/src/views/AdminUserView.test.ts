import { reactive } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { expect, it, vi } from 'vitest'
import AdminUserProfilePage from '@/components/admin/users/AdminUserProfilePage.vue'
const route = reactive({ params: { userId: 'first-member' } })
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ push: vi.fn() }) }))
import AdminUserView from './AdminUserView.vue'

it('replaces the whole profile editor when navigating to another member', async () => {
  const wrapper = shallowMount(AdminUserView)
  const original = wrapper.getComponent(AdminUserProfilePage).vm
  route.params.userId = 'second-member'
  await wrapper.vm.$nextTick()
  expect(wrapper.getComponent(AdminUserProfilePage).props('userId')).toBe('second-member')
  expect(wrapper.getComponent(AdminUserProfilePage).vm).not.toBe(original)
  wrapper.unmount()
})
