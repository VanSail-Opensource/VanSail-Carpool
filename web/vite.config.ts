import { fileURLToPath, URL } from 'node:url'
import { readdirSync, readFileSync } from 'node:fs'
import { writeFile } from 'node:fs/promises'

import ui from '@nuxt/ui/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

import { gameIconRegistry } from './src/components/activity/gameIcons.ts'
import { agreementIconRegistry } from './src/components/onboarding/agreementIcons.ts'

const uiIconRegistry = {
  arrowDown: 'i-ph-arrow-down', arrowLeft: 'i-ph-arrow-left',
  arrowRight: 'i-ph-arrow-right', arrowUp: 'i-ph-arrow-up',
  caution: 'i-ph-warning', check: 'i-ph-check',
  chevronDoubleLeft: 'i-ph-caret-double-left', chevronDoubleRight: 'i-ph-caret-double-right',
  chevronDown: 'i-ph-caret-down', chevronLeft: 'i-ph-caret-left',
  chevronRight: 'i-ph-caret-right', chevronUp: 'i-ph-caret-up',
  close: 'i-ph-x', copy: 'i-ph-copy', copyCheck: 'i-ph-check',
  dark: 'i-ph-moon', drag: 'i-ph-dots-six-vertical', ellipsis: 'i-ph-dots-three',
  error: 'i-ph-warning-circle', external: 'i-ph-arrow-square-out', eye: 'i-ph-eye',
  eyeOff: 'i-ph-eye-slash', file: 'i-ph-file', folder: 'i-ph-folder',
  folderOpen: 'i-ph-folder-open', hash: 'i-ph-hash', info: 'i-ph-info',
  light: 'i-ph-sun', loading: 'i-ph-spinner-gap', menu: 'i-ph-list',
  minus: 'i-ph-minus', panelClose: 'i-ph-sidebar-simple', panelOpen: 'i-ph-sidebar-simple',
  plus: 'i-ph-plus', reload: 'i-ph-arrows-clockwise', search: 'i-ph-magnifying-glass', mapPin: 'i-ph-map-pin',
  star: 'i-ph-star', stop: 'i-ph-stop', success: 'i-ph-check-circle',
  system: 'i-ph-monitor', tip: 'i-ph-lightbulb', upload: 'i-ph-upload-simple',
  warning: 'i-ph-warning-circle',
} as const

const explicitlyBundledIcons = [
  ...new Set([
    ...Object.values(uiIconRegistry),
    ...Object.values(gameIconRegistry),
    ...Object.values(agreementIconRegistry),
    'i-lucide-infinity', 'i-lucide-user', 'i-lucide-user-plus', 'i-lucide-users', 'i-lucide-user-x',
  ]),
]

function localeLeafKeys(value: unknown, prefix = ''): string[] {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return prefix ? [prefix] : []
  return Object.entries(value).flatMap(([key, child]) => localeLeafKeys(child, prefix ? `${prefix}.${key}` : key))
}

function localeLeaves(value: unknown, prefix = '', output: Record<string, string> = {}): Record<string, string> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    if (typeof value === 'string') output[prefix] = value
    return output
  }
  for (const [key, child] of Object.entries(value)) localeLeaves(child, prefix ? `${prefix}.${key}` : key, output)
  return output
}

function validateLocaleParity(): void {
  const read = (name: string) => {
    const directory = new URL(`./locales/${name}/`, import.meta.url)
    return readdirSync(directory)
      .filter((file) => file.endsWith('.json'))
      .sort()
      .reduce<Record<string, unknown>>((messages, file) => ({
        ...messages,
        ...JSON.parse(readFileSync(new URL(file, directory), 'utf8')) as Record<string, unknown>,
      }), {})
  }
  const english = new Set(localeLeafKeys(read('en')))
  const englishLeaves = localeLeaves(read('en'))
  for (const name of ['zh-CN']) {
    const actual = new Set(localeLeafKeys(read(name)))
    const missing = [...english].filter((key) => !actual.has(key))
    const extra = [...actual].filter((key) => !english.has(key))
    if (missing.length || extra.length) throw new Error(`Locale ${name} does not match en.json. Missing: ${missing.join(', ')}. Extra: ${extra.join(', ')}`)
    const translatedLeaves = localeLeaves(read(name))
    for (const key of english) {
      const expected = [...englishLeaves[key].matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort().join(',')
      const actualPlaceholders = [...translatedLeaves[key].matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort().join(',')
      if (expected !== actualPlaceholders) throw new Error(`Locale ${name} placeholder mismatch at ${key}`)
    }
  }
}

export default defineConfig({
  plugins: [
    vue(),
    ui({
      colorMode: false,
      ui: {
        colors: { primary: 'emerald', neutral: 'zinc' },
        icons: uiIconRegistry,
        button: {
          slots: {
            base: 'min-h-11 min-w-11 justify-center text-center',
            label: 'whitespace-normal text-wrap',
          },
          variants: {
            block: {
              true: {
                trailingIcon: 'ms-0',
              },
            },
          },
        },
        input: { slots: { base: 'min-h-11' } },
        inputNumber: { slots: { base: 'min-h-11 !px-12', increment: 'w-11 justify-center !pe-0', decrement: 'w-11 justify-center !ps-0' } },
        select: {
          slots: {
            base: 'min-h-11', item: 'min-h-11 items-center',
            content: 'tg-glass-menu', arrow: 'tg-glass-arrow',
          },
        },
        selectMenu: {
          slots: {
            base: 'min-h-11', item: 'min-h-11 items-center',
            content: 'tg-glass-menu', arrow: 'tg-glass-arrow',
          },
        },
        modal: {
          slots: {
            overlay: 'tg-modal-overlay tg-glass-backdrop', content: 'tg-modal tg-glass-modal tg-glass-surface',
            header: 'tg-overlay-header', wrapper: 'tg-overlay-copy', footer: 'tg-overlay-actions',
          },
          variants: {
            fullscreen: { true: { content: 'tg-modal--fullscreen' }, false: { content: 'tg-modal--windowed' } },
            scrollable: { true: { overlay: 'tg-modal-overlay--scrollable', content: 'tg-modal--scrollable' } },
          },
        },
        drawer: {
          slots: {
            overlay: 'tg-glass-backdrop', content: 'tg-drawer tg-glass-surface', container: 'tg-drawer__container',
            wrapper: 'tg-overlay-copy', footer: 'tg-overlay-actions tg-drawer__footer',
          },
        },
        slideover: {
          slots: {
            overlay: 'tg-glass-backdrop', content: 'tg-slideover tg-glass-surface', header: 'tg-overlay-header',
            wrapper: 'tg-overlay-copy', footer: 'tg-overlay-actions',
          },
        },
        tooltip: { slots: { content: 'tg-glass-tooltip', arrow: 'tg-glass-arrow' } },
        popover: { slots: { content: 'tg-glass-menu', arrow: 'tg-glass-arrow' } },
        dropdownMenu: { slots: { content: 'tg-glass-menu', arrow: 'tg-glass-arrow' } },
        contextMenu: { slots: { content: 'tg-glass-menu' } },
        navigationMenu: { slots: { viewport: 'tg-glass-menu', arrow: 'tg-glass-navigation-arrow' } },
        toast: { slots: { root: 'tg-glass-toast' } },
      },
      icon: {
        clientBundle: {
          icons: explicitlyBundledIcons,
          scan: {
            globInclude: ['**/*.{vue,ts}'],
          },
          sizeLimitKb: 512,
        },
      },
    }),
    {
      name: 'validate-locale-parity',
      buildStart: validateLocaleParity,
    },
    {
      name: 'preserve-go-embed-placeholder',
      apply: 'build',
      closeBundle: () => writeFile(
        fileURLToPath(new URL('../internal/webui/dist/.placeholder', import.meta.url)),
        'Frontend assets are generated here by the Vite production build.\n',
      ),
    },
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      'lottie-web': fileURLToPath(new URL('./node_modules/lottie-web/build/player/lottie_light.js', import.meta.url)),
    },
  },
  optimizeDeps: {
    entries: ['index.html', 'admin.html', 'subscription.html'],
    include: [
      '@tiptap/core', '@tiptap/vue-3', '@tiptap/markdown', '@tiptap/starter-kit',
      'prosemirror-state', 'prosemirror-view', 'prosemirror-model', 'prosemirror-transform',
      'prosemirror-gapcursor', 'prosemirror-keymap', 'prosemirror-commands', 'prosemirror-history',
      '@tiptap/pm/state', '@tiptap/pm/view', '@tiptap/pm/model', '@tiptap/pm/transform',
      '@tiptap/pm/gapcursor', '@tiptap/pm/keymap', '@tiptap/pm/commands', '@tiptap/pm/history',
    ],
  },
  build: {
    rolldownOptions: { input: { main: fileURLToPath(new URL('./index.html', import.meta.url)), admin: fileURLToPath(new URL('./admin.html', import.meta.url)), subscription: fileURLToPath(new URL('./subscription.html', import.meta.url)) } },
    target: 'es2020',
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
    sourcemap: false,
    assetsDir: 'assets',
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/healthz': 'http://127.0.0.1:8080',
      '/readyz': 'http://127.0.0.1:8080',
    },
  },
  test: {
    // Vitest 5 changed the default; retain the existing mock lifecycle.
    clearMocks: false,
    environment: 'happy-dom',
    setupFiles: ['./src/test/storage-setup.ts', './src/test/setup.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      include: ['src/composables/**/*.ts', 'src/router/**/*.ts', 'src/stores/**/*.ts', 'src/utils/**/*.ts'],
    },
  },
})
