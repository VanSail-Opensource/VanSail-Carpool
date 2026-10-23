# Squad profiles

## Ownership

This module owns local, customer-facing metadata for live X-Node internal
squads. X-Node remains authoritative for squad UUIDs, names, and presence;
this module never edits upstream squad or node configuration.

## Profile contract

Each saved profile is one of `broadband`, `china_optimized`, or
`international_network`. Broadband stores ISP, positive whole-number Mbps,
static/dynamic mode, and free-form detailed location. The other two types store
carrier details, nullable unlimited-or-Mbps port speed, and an ISO alpha-2
country code. International profiles also require at least one upstream carrier.

The existing local `description` column remains additional Markdown. Generated
facts are projected from the typed profile at runtime, so localized labels are
not duplicated in SQLite or the API payload.

## Boundaries and failure behavior

The Go profile package normalizes whitespace, country casing, carrier lists, and
irrelevant fields before persistence. Invalid or incomplete admin writes return
`INVALID_SQUAD_PROFILE`. Legacy overrides may have a null profile and are
configured through the editor before they can be saved again.

All upstream reads continue through the existing X-Node queue. Database
migration `017_squad_profiles.sql` is additive and leaves deployed migrations
immutable. Vue uses Nuxt UI controls and locale-owned text. The member catalog
summary adds the squad name plus a profile-specific icon and restrained
semantic color treatment. Standalone catalog cards own that color across their
outer surface, while the compact default summary remains suitable for
administration.

## Verification

Contract generation, structure/README audits, locale audits, typechecking,
linting, Go formatting, and diff inspection are required. Local runtime tests
are intentionally omitted for this repository workflow.
