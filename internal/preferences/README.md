# User preferences

The compatibility `activeCombo` field also admits members with actual PAYG
purchases, derived from existing lifetime purchased-byte evidence. Community
invites, Around TX and activity entrance gates share this rule. Real combo,
add-on, automatic-renewal and PAYG debits enable Show Around TX in the same
transaction. Later manual opt-outs remain effective until another qualifying
purchase. Upgrade backfills the entrance for existing spenders; unrelated
notification and PAYG choices are preserved. Boost checks remain required.

`types.go` owns defaults, typed partial updates, snapshots, and the delivery reader.
`delivery.go` maps every automatic private notification to exactly one category.
The database stores ten preference flags; eligibility, language, currency, reset
automation, and Telegram tags retain their existing sources of truth. Unknown
events fail category resolution so new producers must explicitly choose a category.
`delivery_test.go` verifies category isolation for all current events.

PAYG access defaults on. The master flag is an account choice; the snapshot adds a sparse `paygSquads` opt-out map (missing means enabled). PATCH merges supplied squad entries and enabling removes the stored opt-out. Combo controls remain independent.
