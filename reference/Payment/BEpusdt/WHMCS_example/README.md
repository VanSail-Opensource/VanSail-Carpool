# Archived BEpusdt WHMCS sample

This directory is protocol reference material only. It is excluded from the
Carpool image and is not a supported WHMCS gateway. The PHP sources have `.php.txt`
extensions so they are not deployable PHP entry points. Their internal include
and callback paths retain the original upstream filenames for historical reading.

Do not copy this archive into a web root or WHMCS `modules/gateways` directory.
Use Carpool's maintained Go payment integration and retained order recovery path.

## Contents and provenance

- `epusdt.php.txt`: upstream gateway settings, invoice request and payment redirect.
- `callback/epusdt_notify.php.txt`: upstream signed callback/invoice credit sketch.
- `callback/epusdt_return.php.txt`: upstream browser return redirect.
- `epusdt/lib/epusdt.php.txt`: upstream HTTP client and MD5 protocol signatures.
- `epusdt/whmcs.json`: original manifest, including its MIT license declaration,
  author fields and upstream URLs.
- `README.upstream.md`: preserved original overview and installation text. It is
  historical documentation, not installation guidance for this repository.

The manifest identifies <https://github.com/v03413/whcms-gateway-bepusdt> and
<https://github.com/v03413/BEpusdt>. The imported files did not pin a source
revision; this archive does not assert compatibility with later provider versions.

## Known unsafe or incomplete assumptions

The sample credits a callback amount without a retained local invoice/amount/
currency binding, interpolates the provider payment URL into JavaScript without
escaping or a URL scheme restriction, and presents a raw provider error as HTML.
Its callback also uses `money` while the copied API document describes `amount`.
It is not suitable as a starting point for a live financial integration.

The text archive retains these behaviors to explain the historical protocol.
No production callback, payment amount, retry policy or gateway is changed by
retiring these samples. See [the payment reference inventory](../../README.md).
