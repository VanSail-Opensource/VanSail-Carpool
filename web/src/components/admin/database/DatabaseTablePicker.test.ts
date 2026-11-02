/* eslint-disable vue/one-component-per-file */

import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { DatabaseTable } from '@/api/features'
import DatabaseTablePicker from './DatabaseTablePicker.vue'

const tables: DatabaseTable[] = [
  { name: 'users', columns: [], highRisk: false, warning: '' },
  { name: 'audit_events_with_a_long_name', columns: [], highRisk: true, warning: '' },
]
const SelectMenuStub = defineComponent({
  inheritAttrs: false,
  props: { searchInput: { type: Object, default: () => ({}) } },
  emits: ['update:modelValue'],
  template: `<div role="combobox" :data-search-placeholder="searchInput.placeholder" @click="$emit('update:modelValue', 'audit_events_with_a_long_name')" />`,
})
const InputStub = defineComponent({
  emits: ['update:modelValue'],
  template: `<div role="searchbox" @click="$emit('update:modelValue', 'audit')" />`,
})
const ButtonStub = defineComponent({
  inheritAttrs: false,
  emits: ['click'],
  template: `<div role="button" data-test="table-option" @click="$emit('click')"><slot /></div>`,
})

describe('DatabaseTablePicker', () => {
  it('provides a searchable phone picker with typed selection', async () => {
    const wrapper = mount(DatabaseTablePicker, {
      props: { tables, selected: 'users', busy: false },
      global: { stubs: { SelectMenu: SelectMenuStub, Input: InputStub, Button: ButtonStub, FormField: { template: '<div><slot /></div>' } } },
    })

    expect(wrapper.get('[role="combobox"]').attributes('data-search-placeholder')).toBe('Search table names')
    await wrapper.get('[role="combobox"]').trigger('click')
    expect(wrapper.emitted('select')).toEqual([['audit_events_with_a_long_name']])
  })

  it('retains searchable desktop table options', async () => {
    const wrapper = mount(DatabaseTablePicker, {
      props: { tables, selected: 'users', busy: false },
      global: { stubs: { SelectMenu: SelectMenuStub, Input: InputStub, Button: ButtonStub, FormField: { template: '<div><slot /></div>' } } },
    })

    expect(wrapper.findAll('[data-test="table-option"]')).toHaveLength(2)
    await wrapper.get('[role="searchbox"]').trigger('click')
    expect(wrapper.findAll('[data-test="table-option"]')).toHaveLength(1)
    expect(wrapper.get('[data-test="table-option"]').text()).toContain('audit_events_with_a_long_name')
  })
})
