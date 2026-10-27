# Charts

Quantitative charts use the official [shadcn-vue chart composition](https://www.shadcn-vue.com/docs/components/chart), adapted from its MIT-licensed source (unovue/shadcn-vue, 2026-10-27). Unovis 1.7.1 owns chart rendering. Routing/BGP diagrams and uptime timelines stay in their existing modules.

- `index.ts` provides the typed configuration/context and official Unovis aliases.
- `ChartContainer.vue` scopes chart colors and consumes the current experience theme without importing either root.
- `ChartTooltipContent.vue` formats escaped Vue text. `tooltip.ts` serializes and immediately releases detached trees; no owner data is globally cached.
- `ChartLegendContent.vue` displays series labels and solid/dashed styles.
- `LineChart.vue` composes Unovis lines, axes, intervals and crosshairs. Missing values break lines by default; callers may opt into gap connectors and a dashed bridge between series without changing inspection values or intervals. Touch selection, arrow/Home/End keys and visible previous/next controls expose the same values. Callers retain exact integer evidence and provide display conversions only.

Extension points are numerical series, localized labels, value formatters and optional intervals. These components never fetch data, calculate prices or persist telemetry.

Source copyright (c) 2023-present shadcn-vue contributors. Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files, to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies, and to permit persons to whom the Software is furnished to do so, subject to inclusion of this copyright and permission notice. THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY ARISING FROM THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
