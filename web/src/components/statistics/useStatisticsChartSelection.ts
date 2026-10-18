import { computed, shallowRef, type ComputedRef } from 'vue'


export interface StatisticsChartSelectable {
  interactionId: string
}

export function useStatisticsChartSelection<T extends StatisticsChartSelectable>(items: ComputedRef<readonly T[]>) {
  const hoveredId = shallowRef<string>()
  const selectedId = shallowRef<string>()
  const explicitActiveItem = computed(() => {
    const activeId = hoveredId.value ?? selectedId.value
    return items.value.find((item) => item.interactionId === activeId)
  })
  const hasActive = computed(() => explicitActiveItem.value !== undefined)
  const activeItem = computed(() => explicitActiveItem.value ?? items.value[0])

  function activate(interactionId: string): void {
    hoveredId.value = interactionId
  }

  function deactivate(interactionId: string): void {
    if (hoveredId.value === interactionId) hoveredId.value = undefined
  }

  function select(interactionId: string): void {
    selectedId.value = selectedId.value === interactionId ? undefined : interactionId
  }

  function isActive(interactionId: string): boolean {
    return activeItem.value?.interactionId === interactionId
  }

  function isSelected(interactionId: string): boolean {
    return selectedId.value === interactionId
  }

  return { activeItem, hasActive, activate, deactivate, select, isActive, isSelected }
}
