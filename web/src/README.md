# Frontend source

- `App.vue` composes the application shell and mounts the active route immediately, so cached content is not delayed by exit or entry transitions. Route keys retain the existing reset behavior.
- `env.d.ts` declares Vite and Vue build-time types.
- `main.ts` selects browser or member Mini App and loads the Telegram SDK only for Mini App launches.
- `miniapp.ts` installs member WebView compatibility, Telegram host behavior, and member application plugins.
- `browser/` owns the independent public entrance and desktop-first admin console.
