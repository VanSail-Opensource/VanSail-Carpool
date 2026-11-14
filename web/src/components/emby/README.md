# Emby components

- `EmbyPage.vue` composes account setup and preference states with animated state replacement and retry feedback.
- `EmbySetupPanel.vue` provisions a paid Emby account with paired desktop fields, a single-column phone flow, selection feedback, and rigid confirmation feedback.
- `EmbyPreferencesPanel.vue` updates restrictions and passwords in balanced desktop panels with selection and confirmation feedback.
- `EmbyLibraryPicker.vue` selects disabled libraries and emits haptics only when a library value changes.
- `EmbyLibraryPicker.vue` also owns the shared selected-library indicator; setup and preference panels remain form-first and static.

Library names wrap inside their shrinkable column, keeping the toggle reachable for long unbroken names on narrow phones.
