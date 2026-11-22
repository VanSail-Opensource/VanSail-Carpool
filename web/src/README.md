# Frontend source

- `App.vue` composes the application shell and mounts the active route immediately, so cached content is not delayed by exit or entry transitions. Private shell and immersive routes are also keyed by authenticated owner, retiring the prior member's dashboards, dialogs and request lifecycles on identity replacement.
- `env.d.ts` declares Vite and Vue build-time types.
- `main.ts` selects the member Mini App, browser console, or public subscription entry. The member SDK and entry module load in parallel. There is no pre-mount splash; fatal bootstrap errors retain the localized native reload action.
- `main.test.ts` covers entry isolation, parallel SDK loading, and recovery from an unavailable SDK.
- `miniapp.ts` installs compatibility, starts the existing cookie session probe as soon as its Pinia store loads, and prepares the real shell in parallel. The same store waits for host readiness before Telegram login and route access.
- `browser/` owns the independent public entrance and desktop-first admin console.
