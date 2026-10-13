/* eslint-disable vue/one-component-per-file */

import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DatabaseQueryControls from './DatabaseQueryControls.vue'
import type { DatabaseColumnOption, DatabaseOperatorOption, TextDatabaseFilter } from './types'

const columns: DatabaseColumnOption[] = [{ value: 'status', label: 'status' }]
const operators: DatabaseOperatorOption[] = [{ value: 'eq', label: 'Equals' }]
const initial: TextDatabaseFilter[] = [{ column: 'status', operator: 'eq', value: 'active' }]
const DrawerStub = defineComponent({
  props: { open: { type: Boolean, default: false }, ui: { type: Object, default: () => ({}) } },
  emits: ['update:open'],
  template: `<div class="drawer" :data-open="open" :data-body-ui="ui.body"><slot name="close" /><slot name="body" /><footer><slot name="footer" /></footer></div>`,
})
const ButtonStub = defineComponent({
  inheritAttrs: false,
  props: { icon: { type: String, default: '' }, label: { type: String, default: '' }, disabled: { type: Boolean, default: false } },
  emits: ['click'],
  template: `<div role="button" :class="$attrs.class" :data-icon="icon" :data-label="label" :data-disabled="disabled || undefined" @click="$emit('click')">{{ label }}</div>`,
})
const FilterFieldsStub = defineComponent({
  inheritAttrs: false,
  props: { filters: { type: Array, required: true } },
  emits: ['update:filters'],
  template: `<div v-bind="$attrs" role="button" data-test="filter-editor" :data-count="filters.length" @click="$emit('update:filters', [...filters, { column: 'status', operator: 'eq', value: 'pending' }])" />`,
})

function mountControls(filters: TextDatabaseFilter[] = initial) {
  return mount(DatabaseQueryControls, {
    props: { search: '', filters, columnItems: columns, operators },
    global: { stubs: { Slideover: DrawerStub, Button: ButtonStub, Input: { template: '<div />' }, DatabaseFilterFields: FilterFieldsStub } },
  })
}

describe('DatabaseQueryControls', () => {
  it('keeps phone filter edits local until Apply and discards Cancel edits', async () => {
    const wrapper = mountControls()
    expect(wrapper.get('[data-icon="i-ph-funnel"]').attributes('data-label')).toBe('Filters (1)')
    await wrapper.get('[data-icon="i-ph-funnel"]').trigger('click')
    const editor = wrapper.get('.drawer [data-test="filter-editor"]')
    expect(editor.attributes('data-count')).toBe('1')
    await editor.trigger('click')
    expect(wrapper.emitted('update:filters')).toBeUndefined()
    await wrapper.get('[data-label="Cancel"]').trigger('click')
    await wrapper.get('[data-icon="i-ph-funnel"]').trigger('click')
    expect(wrapper.get('.drawer [data-test="filter-editor"]').attributes('data-count')).toBe('1')
    await wrapper.get('.drawer [data-test="filter-editor"]').trigger('click')
    await wrapper.get('[data-label="Apply filters"]').trigger('click')
    expect(wrapper.emitted('update:filters')?.[0]?.[0]).toHaveLength(2)
  })

  it('keeps Clear local until Apply', async () => {
    const wrapper = mountControls()
    await wrapper.get('[data-icon="i-ph-funnel"]').trigger('click')
    await wrapper.get('[data-label="Clear filters"]').trigger('click')
    expect(wrapper.emitted('update:filters')).toBeUndefined()
    await wrapper.get('[data-label="Apply filters"]').trigger('click')
    expect(wrapper.emitted('update:filters')?.[0]?.[0]).toEqual([])
  })

  it('enforces the five-filter limit and body-only drawer scrolling', async () => {
    const filters = Array.from({ length: 5 }, (_, index) => ({ column: 'status', operator: 'eq' as const, value: String(index) }))
    const wrapper = mountControls(filters)
    await wrapper.get('[data-icon="i-ph-funnel"]').trigger('click')
    expect(wrapper.get('.database-query__mobile-add').attributes('data-disabled')).toBe('true')
    expect(wrapper.get('.drawer').attributes('data-body-ui')).toBe('database-filter-drawer__body')
  })
})
