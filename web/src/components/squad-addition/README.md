# Squad additions

The dialog opens from the Settings subscription action and reloads if its active
purchase changes. Native 160ms step and presence fades honor reduced motion.
The regression in `SquadAdditionDialog.test.ts` protects the initial-open path.

Owns the two-step active-ride add-on dialog. `SquadAdditionDialog.vue` orchestrates selection, activation codes, checkout, and in-place confirmation; `SquadAdditionCheckout.vue` renders the price review and completion state. `SquadAdditionDialog.test.ts` protects the one-based flow to zero-based progress mapping. The module reuses the catalog squad picker, whose selectable squad cards use the configured display currency directly, and `useSquadAddition` for API state. The checkout review retains its dual-currency presentation.

The visual stepper keeps its zero-based component model; a localized screen-reader status announces the one-based stage and label. Lost purchase replies retain selection and retry the same request key through useSquadAddition.
