# Product language catalog

`admin-endpoints.json` owns protocol-aware endpoint editing, diagnostics and
administrator field labels independently from generated member `diy-fields.json`.

English and Simplified Chinese are split into matching domain JSON files so
copy stays reviewable and below the repository line ceiling. `connections.json`
owns active IP observations and exact-address grants; `home.json` owns the adjustable UTC per-node traffic labels; `community.json` owns group and channel access copy. `manifest.json`
declares the supported locales and ordered domains, including global render recovery, Home, the visual
onboarding editor, and audited administrator workflows. The structure gate checks
every domain is declared. It and Vite validate key and placeholder parity within
each file, so domains that extend the same namespace cannot hide missing keys.

`ip-lookup-checks.json` owns proxy inventories, background reminders, subnet coverage, full port probes and refresh quota copy.

`en/member-nodes.json` and `zh-CN/member-nodes.json` localize live usage, limitations and member routing controls.

`node-forms.json` owns matching English/Chinese operational form, source-filter, interval, provider labels.
