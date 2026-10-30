# Interface font assets

Local font files for the independent browser admin and member interfaces. This
directory supplies assets only; each experience owns its font declarations and
theme integration. Public URLs begin with `/fonts/`.

| File | Family and available variants | Provenance |
| --- | --- | --- |
| `Geist-latin.woff2` | Geist 1.800, upright variable weight 100–900 | Reused existing project asset; official family: `vercel/geist-font` |
| `NotoSansSC-variable.ttf` | Noto Sans SC 2.004, upright variable weight 100–900 | Unmodified Google Fonts file at commit `a85815a42757630ce188fdad368c2dfc444d4773` |
| `Geist-license.json` | Complete SIL OFL 1.1 notice and font metadata | Preserves the original `../landing/Geist-OFL.txt` text |
| `NotoSansSC-license.json` | Complete SIL OFL 1.1 notice and font metadata | Preserves the downloaded `NotoSansSC-OFL.txt` text |
| `NotoSansSC-OFL.txt` | Original upstream license | Downloaded unchanged from the same official Google Fonts family |

The license JSON files contain source URLs, SHA-256 hashes, concrete font files,
actual weight ranges, consuming blueprint sections, and acquisition evidence.
Both fonts serve `admin_entry` and `member_entrance`. Approved interface weights
are 400, 500, 600 and 700. No italic variant is approved or supplied.

## Verification and integration limits

Binary table inspection confirmed both family names and weight axes. Geist covers
all 95 printable ASCII characters. Noto Sans SC covers the loading text and all
901 distinct CJK characters and full-width punctuation found in the 47 existing
Simplified Chinese locale JSON files.

An isolated Chrome DevTools font specimen loaded both local files with HTTP 200,
awaited `document.fonts.ready`, and confirmed matching loaded FontFace entries
and `document.fonts.check` at every approved weight. The rendered specimens were
visually inspected. Its directory page also requested a missing favicon; that
404 was unrelated to the font files. This does not validate application font
integration, loading-state behavior, mobile layout, or motion.

The complete Chinese font is 17,772,300 bytes. Integrate it asynchronously with
`font-display: swap`; font readiness must never gate authentication or interaction.
Retain Chinese system fallbacks and existing monospace stacks. Recheck fonts in
both application roots during Build. No application CSS, components, or
dependencies were changed during acquisition.

When replacing a font, retain its applicable license, update the file hashes and
source revision, and recheck family names, weight axes, and locale coverage. The
approved loading design uses live typography; reference photographs and icons
do not require new asset files for these two sections.
