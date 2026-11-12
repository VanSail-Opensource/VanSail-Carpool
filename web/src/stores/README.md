# Stores

The session store binds the response cache to the authenticated user, role and
onboarding state. A new identity starts the bounded member/admin preload queue;
clearing or failing authentication cancels preload and discards cached responses.

- `session.ts` owns authenticated session bootstrap and derived user state.
  It retains the stable authentication error code alongside localized copy so
  verified cleanup recovery can use its own heading without opening a session.
  Concurrent route guards await the same bootstrap; forced refreshes, explicit
  session updates and clearing invalidate older replies and their error paths.
- `sessionConcurrency.test.ts` covers awaited shared reads and stale response
  isolation across refresh, completion and clear boundaries.
- `sessionRecovery.test.ts` covers verified cleanup error classification,
  unauthenticated state, and removal of that classification on a later retry.

`preferences.ts` scopes preferences to the authenticated account, serializes partial saves and rejects stale load/save responses.
`preferences.test.ts` covers failed saves and stale responses after account changes.
