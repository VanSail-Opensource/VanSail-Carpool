# Visual system

- `page-transitions.css` provides the clipped route viewport while Motion owns the keyed outgoing/incoming handoff: local horizontal movement on phones and local vertical movement on wide screens. Reduced motion removes translation and retains a short opacity change.

`main.css` is the ordered entry point for these cascade layers:

- `foundation-01.css` defines tokens, document defaults, typography, and the semantic mint primary color. Native resets preserve Nuxt UI foreground utilities so solid, outline, and ghost controls retain readable contrast.
- `foundation-02.css` defines shared controls, surfaces, and utility patterns.
- `shell-01.css` styles the application shell and primary navigation.
- `shell-02.css` styles page framing, notices, and session states.
- `shell-03.css` styles the fullscreen-only localized greeting and its safe-area reserve.
- `onboarding-01.css` styles onboarding structure and progress.
- `onboarding-02.css` styles onboarding forms and agreements.
- `onboarding-03.css` styles onboarding completion and supporting states.
- `session-01.css` owns the responsive loading wordmark, outline-to-fill letter reveal, Chinese-to-English replacement, and continuous loading keyframes. `SessionEntrance.vue` owns the keyed Motion handoff and `useLoadingSequence.ts` coordinates its 1.2-second introduction.
- `dashboard-01.css` styles dashboard summaries and account details.
- `dashboard-02.css` styles dashboard lists, actions, and secondary states.
- `home-01.css` styles the wrapping balance/action row, paired Home language/currency controls, 44px Home actions, narrow-phone subscription layout, Nuxt UI usage-progress detail, and traffic surfaces.
- `home-02.css` styles traffic details, ride facts, and Around TX links on Home.
- `home-03.css` styles the compact renewal term slider and separated quote total/date block.
- `home-05.css` keeps Home's card stream and alerts in the same single-column order on phones and desktop, with shrinkable grid tracks for long ride names.
- `connections-01.css` styles the connections page heading, node/IP rows, empty states, and selected connection confirmation detail.
- `connections-02.css` styles the stable connection scan visualization, progress presentation, tablet split layout, and reduced-motion fallback.
- `connections-03.css` styles active block rows, exact-expiry metadata, and compact unblock confirmation targets.
- `statistics-01.css` styles the independently divided statistics sections, centered refresh control, KPI panels, dense icon-based node rows, and compact states.
- `statistics-02.css` styles the responsive generic SVG donut, labeled pies, traffic tooltips, legends, and normalized stacked bars.
- `affiliates-01.css` styles flat member metrics, tier progress, referral rows, and tablet expansion.
- `catalog-01.css` styles catalog browsing and plan presentation, including variable-height responsive columns for optional squads.
- `catalog-02.css` styles catalog selection and checkout details.
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

The system is dark-only, flat, gradient-free, and uses borders instead of decorative shadows.

Route roots render through one keyed Motion presence boundary so restoring cached
data does not replay a full-page entrance. List-level layout/presence and action
feedback remain local to their components. CSS keyframes are reserved for
perpetual loaders, skeletons, radar, and particles.

## Admin layout

- `admin-08.css` owns the shared admin row geometry and final responsive heading/card refinements. It is imported after the earlier admin and responsive layers so section-specific row variants can correct their actual content tracks.

Active Combo flip-card layout and its prediction reverse: `home-04.css`.
