import { h, render } from 'vue'
import type { Component } from 'vue'
import type { ChartConfig } from './index'
// Render Vue text nodes so upstream/user labels cannot become tooltip HTML.
// No global cache: owner-specific payloads and detached Vue trees are released.
export function componentToString(config: ChartConfig, component: Component, props: Record<string, unknown> = {}) {
  return (datum: unknown, x: number | Date): string => {
    const payload = datum && typeof datum === 'object' && 'data' in datum ? datum.data : datum
    const element = document.createElement('div')
    render(h(component, { ...props, config, payload, x }), element)
    const html = element.innerHTML
    render(null, element)
    return html
  }
}
