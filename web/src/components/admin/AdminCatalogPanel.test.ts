import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { Combo } from '@/api/types'
import { t } from '@/i18n'
import AdminCatalogPanel from './AdminCatalogPanel.vue'
import AdminCatalogEditor from './AdminCatalogEditor.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

const combos = ['first', 'second'].map(id => ({ id, name: id } as Combo))
function render() {
  return mount(AdminCatalogPanel, { global: { stubs: {
    AdminSectionState: { template: '<div><slot /></div>' },
    Table: { props: ['data'], template: '<div><slot v-for="item in data" name="actions-cell" :row="{ original: item }" /></div>' },
    AdminCatalogEditor: { props: ['combo', 'busy'], emits: ['save', 'cancel'], template: '<div />' },
    AdminSquadEditor: true,
    ConfirmDialog: { props: ['open', 'busy'], emits: ['confirm', 'update:open'], template: '<div />' },
  } } })
}
beforeEach(() => vi.spyOn(api, 'getAdminResource').mockImplementation(async resource => ({ items: resource === 'combos' ? combos : [] }) as never))
afterEach(() => vi.restoreAllMocks())

describe('catalog mutation review ownership', () => {
  it('locks the reviewed editor and admits one save while its reply is pending', async () => {
    let finish!: () => void
    const update = vi.spyOn(api, 'updateAdminResource').mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve }) as never)
    const wrapper = render()
    await flushPromises()
    await wrapper.get(`button[aria-label="${t('adminCatalog.editNamed', { name: 'first' })}"]`).trigger('click')
    const editor = wrapper.getComponent(AdminCatalogEditor)
    editor.vm.$emit('save', { name: 'reviewed' })
    editor.vm.$emit('save', { name: 'duplicate' })
    editor.vm.$emit('cancel')
    await flushPromises()
    expect(editor.props('combo')?.id).toBe('first')
    expect(editor.props('busy')).toBe(true)
    expect(update).toHaveBeenCalledExactlyOnceWith('combos', 'first', { name: 'reviewed' })
    const other = wrapper.get(`button[aria-label="${t('adminCatalog.editNamed', { name: 'second' })}"]`)
    expect(other.attributes('disabled')).toBeDefined()
    finish()
    await flushPromises()
    expect(wrapper.findComponent(AdminCatalogEditor).exists()).toBe(false)
    wrapper.unmount()
  })

  it('retains the confirmed hide target during submission', async () => {
    let finish!: () => void
    const remove = vi.spyOn(api, 'deleteAdminResource').mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve }))
    const wrapper = render()
    await flushPromises()
    await wrapper.get(`button[aria-label="${t('adminCatalog.hideNamed', { name: 'first' })}"]`).trigger('click')
    const dialog = wrapper.getComponent(ConfirmDialog)
    dialog.vm.$emit('confirm')
    dialog.vm.$emit('confirm')
    dialog.vm.$emit('update:open', false)
    await flushPromises()
    expect(dialog.props('open')).toBe(true)
    expect(dialog.props('busy')).toBe(true)
    expect(remove).toHaveBeenCalledExactlyOnceWith('combos', 'first')
    finish()
    await flushPromises()
    expect(dialog.props('open')).toBe(false)
    wrapper.unmount()
  })
})
