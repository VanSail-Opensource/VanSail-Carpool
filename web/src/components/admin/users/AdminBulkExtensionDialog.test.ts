import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import UButton from '@nuxt/ui/runtime/components/Button.vue'
import USelectMenu from '@nuxt/ui/runtime/components/SelectMenu.vue'
import { adminOperationsApi, type BulkExtensionPreview } from '@/api/adminOperations'
import { ApiError } from '@/api/http'
import { t } from '@/i18n'
import AdminBulkExtensionDialog from './AdminBulkExtensionDialog.vue'

vi.mock('motion-v', async importOriginal => ({ ...await importOriginal<typeof import('motion-v')>(), AnimatePresence: { template: '<div><slot /></div>' }, motion: { div: { template: '<div><slot /></div>' } } }))
afterEach(() => vi.restoreAllMocks())
beforeEach(() => vi.spyOn(adminOperationsApi, 'getCatalogOptions').mockResolvedValue({ combos: [], squads: [] }))
const preview = { matchedUsers: 3, activePurchases: 3, queuedSuccessors: 0 } as BulkExtensionPreview

async function render() {
  const wrapper = shallowMount(AdminBulkExtensionDialog, { props: { open: false }, global: {
    renderStubDefaultSlot: true,
    stubs: { Modal: { props: ['open'], template: '<section v-if="open"><slot name="body" /><slot name="footer" /></section>' } },
  } })
  await wrapper.setProps({ open: true })
  await flushPromises()
  wrapper.findAllComponents(USelectMenu)[0]!.vm.$emit('update:modelValue', ['first-combo'])
  await wrapper.vm.$nextTick()
  const button = (label: string) => wrapper.findAllComponents(UButton).find(item => item.props('label') === t(label))!
  return { wrapper, button }
}

describe('bulk extension preview and retry identity', () => {
  it('rejects a preview that arrives after its filters changed', async () => {
    let finish!: (value: BulkExtensionPreview) => void
    vi.spyOn(adminOperationsApi, 'previewBulkExtension').mockReturnValue(new Promise(resolve => { finish = resolve }))
    const { wrapper, button } = await render()
    button('adminBulkExtension.preview').vm.$emit('click')
    await wrapper.vm.$nextTick()
    wrapper.findAllComponents(USelectMenu)[0]!.vm.$emit('update:modelValue', ['second-combo'])
    await wrapper.vm.$nextTick()
    finish(preview); await flushPromises()
    expect(wrapper.find('.admin-preview-band').exists()).toBe(false)
    expect(button('adminBulkExtension.queue').props('disabled')).toBe(true)
    wrapper.unmount()
  })

  it('preserves the accepted-intent key after ambiguous HTTP errors', async () => {
    vi.spyOn(adminOperationsApi, 'previewBulkExtension').mockResolvedValue(preview)
    const create = vi.spyOn(adminOperationsApi, 'createBulkExtension').mockRejectedValue(new ApiError(503, { code: 'TEMPORARY_FAILURE', message: '' }))
    const { wrapper, button } = await render()
    button('adminBulkExtension.preview').vm.$emit('click'); await flushPromises()
    wrapper.getComponent({ name: 'Textarea' }).vm.$emit('update:modelValue', 'Support adjustment')
    await wrapper.vm.$nextTick()
    button('adminBulkExtension.queue').vm.$emit('click'); await flushPromises()
    button('adminBulkExtension.queue').vm.$emit('click'); await flushPromises()
    expect(create).toHaveBeenCalledTimes(2)
    expect(create.mock.calls[1]![1]).toBe(create.mock.calls[0]![1])
    wrapper.getComponent({ name: 'Textarea' }).vm.$emit('update:modelValue', 'Different adjustment')
    await wrapper.vm.$nextTick()
    button('adminBulkExtension.queue').vm.$emit('click'); await flushPromises()
    expect(create.mock.calls[2]![1]).not.toBe(create.mock.calls[0]![1])
    wrapper.unmount()
  })
})
