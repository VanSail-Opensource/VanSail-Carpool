# Private message administration

Per-user PM controls now live in `components/admin/users/AdminUserPM.vue`.
This directory retains the delivery and repair dialogs shared by that profile.
Data and all user copy come from typed API contracts and English/Chinese localization.

`PMDeliveryDialog.vue` loads recent reference-only receipts on demand, shows read
evidence from member replies and any historical explicit records, cancels stale reads
and surfaces ambiguous delivery without offering an unsafe resend.
`PMTopicRepairDialog.vue` validates decimal Telegram references with Zod and queues
an explicit probe of an existing topic/card. `links.ts` constructs documented forum
and message links; Telegram handles private-group membership at link opening.
`modal.css` suppresses opening/closing transitions when the existing motion preference is reduced.

The backend remains authoritative for panel-admin permissions, recipient eligibility,
moderation, topic uniqueness and safe recovery. Frontend status resets start a new
explicit action; they never replay a previous uncertain message. Dialogs reuse the
existing Nuxt UI modal, notices and motion/accessibility behavior.

Browser PM topic/message links use native browser windows with opener isolation.
The repair dialog renders command failures beside its decimal reference inputs,
so drafts remain available after a failed verification. No Telegram Web App SDK
control participates in browser PM administration.
