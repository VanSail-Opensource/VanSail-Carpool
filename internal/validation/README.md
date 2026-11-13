# Validation package

This package is the shared transport validation boundary.

- `strings.go` defines UTF-8, length, control-character, and regular-expression checks without logging rejected values.
- `request.go` validates HTTP methods, escaped paths, decoded query names and values, and header names and values.
- `json.go` validates every JSON token before the strict destination decoder runs. It rejects duplicate object keys and excessive nesting before materializing a value tree, so typed-decoder object merging cannot hide an earlier invalid string. Distinct case-sensitive map keys remain legal, with every value checked. It permits printable Unicode, passwords, Telegram `initData`, multiline text, and CSV content.
- `validation_test.go` exercises accepted international text and rejected malformed input with table-driven cases. `json_tokens_test.go` covers exact/escaped duplicate keys, nested-object merge behavior, case variants, container depth and malformed trailing input.

Format-specific handlers remain responsible for domain grammars such as UUIDs, money, dates, and CSV column rules.
