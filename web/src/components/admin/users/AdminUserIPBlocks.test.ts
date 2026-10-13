import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import AdminUserIPBlocks from './AdminUserIPBlocks.vue'

const TableStub = defineComponent({
  props: { data: { type: Array, default: () => [] } },
  template: '<div><div v-for="(block, index) in data" :key="index"><slot name="actions-cell" :row="{ original: block }" /></div><slot v-if="!data.length" name="empty" /></div>',
})

describe('AdminUserIPBlocks', () => {
  it('offers unblock but no administrator block control', async () => {
    const wrapper = mount(AdminUserIPBlocks, {
      props: { busy: false, items: [{ id: 'block-1', ip: '203.0.113.4', nodeUuid: 'node', status: 'active',
        createdAt: '2026-08-19T00:00:00Z', expiresAt: '2026-08-22T00:00:00Z' }] },
      global: { stubs: { Table: TableStub, UTable: TableStub, StatusBadge: true, Icon: true,
        Button: { template: '<span data-button @click="$emit(\'click\')">unblock</span>' } } },
    })

    expect(wrapper.findAll('[data-button]')).toHaveLength(1)
    await wrapper.find('[data-button]').trigger('click')
    expect(wrapper.emitted('unblock')?.[0]?.[0]).toMatchObject({ id: 'block-1' })
    expect(wrapper.emitted('block')).toBeUndefined()
  })
})
