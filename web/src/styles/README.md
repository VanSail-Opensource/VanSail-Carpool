# Visual system

- `page-transitions.css` provides the clipped route viewport while Motion owns the keyed outgoing/incoming handoff: local horizontal movement on phones and local vertical movement on wide screens. Reduced motion presents the destination immediately without translation or fading.

`main.css` is the ordered entry point for these cascade layers:

- `foundation-01.css` defines tokens, document defaults, typography, and the restrained cyan primary color and graphite surfaces. Native resets preserve Nuxt UI foreground utilities so solid, outline, and ghost controls retain readable contrast.
- `foundation-02.css` defines shared controls, surfaces, and utility patterns.
- `shell-01.css` styles the application shell and primary navigation. Page padding has a standalone fallback when shell clearance is absent; desktop safe-area calculations also retain valid padding outside the shell.
- `shell-02.css` styles page framing, notices, and session states.
- `shell-03.css` retains only real Telegram/device safe-area insets. The greeting and its fixed-height reserve are removed from desktop and mobile.
- `onboarding-01.css` styles onboarding structure and progress.
- `onboarding-02.css` styles onboarding forms and agreements.
- `onboarding-03.css` styles onboarding completion and supporting states.
- `session-01.css` owns the responsive loading wordmark, bounded 500ms lettering, 450ms Chinese-to-English handoff, and interruptible 160ms cover fade. Readiness never waits for the lettering.
- `member-fonts.css` loads the self-hosted Space Grotesk variable face for the member experience, preserves Chinese/system fallbacks, and fences its stack from browser-only fixture imports.
- `member-surfaces.css` unifies member heading sizes, numerical alignment, card edge lighting, keyboard focus feedback, navigation geometry and utility-page spacing. Filled controls have explicit inverse colors for readable contrast before host theme initialization. Cards use a single 650ms edge sweep on hover, focus or selection, with static touch/keyboard borders; decorative overlays never intercept input. Card travel is limited to selectable surfaces on fine pointers. Reduced motion removes travel and sweeps, and forced colors removes decoration.
- `fonts.css` declares self-hosted variable Geist and Noto Sans SC with nonblocking swap. Both experience roots import font resources independently of their theme rules; technical monospace roles retain their own stack.
- `loading.css` provides the shared 192px indeterminate track using each root's accent tokens, with a static reduced-motion state.
- `dashboard-01.css` styles dashboard summaries and account details.
- `dashboard-02.css` styles dashboard lists, actions, and secondary states.
- `home-01.css` styles the wrapping balance/action row, paired Home language/currency controls, 44px Home actions, narrow-phone subscription layout, Nuxt UI usage-progress detail, and traffic surfaces.
- `home-02.css` styles ride facts and Around TX links on Home.
- `home-03.css` styles the compact renewal term slider and separated quote total/date block.
- `home-05.css` keeps Home's card stream and alerts in the same single-column order on phones and desktop, with shrinkable grid tracks for long ride names.
- `statistics-01.css` styles the independently divided statistics sections, centered refresh control, KPI panels, dense icon-based node rows, and compact states.
- `statistics-02.css` styles the responsive generic SVG donut, labeled pies, traffic tooltips, legends, and normalized stacked bars.
- `affiliates-01.css` styles flat member metrics, tier progress, referral rows, and tablet expansion.
- `catalog-01.css` retains shared link styling and squad-selector spacing; active cards own their responsive presentation.
- `billing-01.css` styles funding controls.
- `billing-02.css` styles provider-payment details.
- `billing-03.css` styles crypto currency/network selection plus the compact payment-instruction rhythm, address copy, QR loading, and fixed countdown.
- `feedback-01.css` styles feedback, empty, and loading states.
- `admin-01.css` styles administrative navigation and section layouts, keeping inset notices within the panel width.
- `admin-02.css` styles administrative forms, lists, and data surfaces.
- `admin-03.css` styles administrative drawers and review workflows.
- `admin-04.css` styles aggregate user profiles, bulk previews, operation resolution, and streamed backup upload.
- `admin-05.css` styles active IP blocks and mobile unblock controls in aggregate user profiles.
- `admin-06.css` styles the compact user-editor shell, responsive action/context rail, profile signals, and overlay form sizing.
- `admin-07.css` styles shared administrative headings, dense rows, and responsive action/control layouts.
- `overlays-01.css` resolves Telegram safe-area bounds while retaining Nuxt UI's native centered position for teleported modals, gives floating surfaces a restrained dark glass treatment with compact, surface-local blur and dim-only backdrops, uses fine inset edge highlights instead of diagonal shine, and preserves responsive drawer/slideover layouts. Its reduced-transparency fallback restores opaque elevated surfaces without changing overlay behavior.
- `motion-01.css` defines shared duration tokens and retains only low-level loading keyframes.
- `motion-02.css` adds reduced-motion-safe tactile press response. Motion owns status arrival, active-navigation feedback, and local semantic transitions in their components.
- `responsive-01.css` owns shared motion keyframes used by compact surfaces.
- `responsive-02.css` expands the phone-first shell into a fixed-width resizable Nuxt UI sticky sidebar desktop layout, and restores wide route, dialog, catalog, settings, and administrative list arrangements.
- `responsive-03.css` applies compact administrative rows below 640px and owns reduced-motion/transparency fallbacks.
- `desktop-01.css` adds rail member context and balanced wide-page widths for desktop while keeping the compact phone shell untouched.

The member system is dark-only with graphite surfaces, restrained cyan accents, 12px panels and 8px controls. Space Grotesk adds geometric character; technical values use the existing monospace stack with tabular figures. Card depth comes from a faint inset edge, with short interaction feedback rather than perpetual decoration. Status colours remain semantic and distinct. The browser console owns a separate theme.

Route roots render through one keyed Motion presence boundary so restoring cached
data does not replay a full-page entrance. List-level layout/presence and action
feedback remain local to their components. CSS keyframes are reserved for
perpetual loaders, skeletons, radar, and particles.

## Admin layout

- `admin-08.css` owns the shared admin row geometry and final responsive heading/card refinements. It is imported after the earlier admin and responsive layers so section-specific row variants can correct their actual content tracks.

Active combo summary layout: `home-04.css`.
