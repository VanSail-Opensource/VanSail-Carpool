# Payment protocol reference archive

These imported documents and source examples support protocol research. They are
excluded by `.dockerignore`; neither the Go server nor the Vite application loads
them. Production payment behavior belongs to `internal/billing`, the provider
adapters and the platform's durable order/ledger transactions.

## Retired executable examples

All ten imported PHP files are archived as `.php.txt`. Do not serve or rename them
to executable scripts. The filenames inside their source remain unchanged as
historical context; they are not current installation instructions.

| Archive | Purpose | Audit disposition |
| --- | --- | --- |
| `EZPay/index.php.txt` | Original sample form | Retired; client input cannot establish an authoritative payable amount |
| `EZPay/epayapi.php.txt` | Sample redirect request | Retired; unsanitized user values flow into generated HTML |
| `EZPay/lib/EpayCore.class.php.txt` | Provider signing and HTTP example | Retired; disabled TLS verification, unescaped HTML attributes and incomplete response/error validation |
| `EZPay/lib/epay.config.php.txt` | Original demo configuration shape | Retired; HTTP endpoint and demo merchant; demo key replaced with a non-credential marker |
| `EZPay/notify_url.php.txt` | Sample asynchronous callback | Retired; comments leave amount/owner binding and duplicate-settlement checks unimplemented |
| `EZPay/return_url.php.txt` | Sample browser return | Retired; unescaped callback status and no durable settlement contract |
| `BEpusdt/WHMCS_example/epusdt.php.txt` | Imported WHMCS gateway | Retired; unsafe provider redirect and raw error output |
| `BEpusdt/WHMCS_example/epusdt/lib/epusdt.php.txt` | Imported signing/HTTP helper | Retired; incomplete endpoint, response and exception handling |
| `BEpusdt/WHMCS_example/callback/epusdt_notify.php.txt` | Imported invoice callback | Retired; invoice/amount/currency binding incomplete and callback field differs from copied API documentation |
| `BEpusdt/WHMCS_example/callback/epusdt_return.php.txt` | Imported browser return | Retired; user-controlled invoice ID appended without validation or encoding |

Original source comments and manifest/license metadata are preserved. EZPay was
imported without a pinned revision or a bundled license declaration; the archive
does not invent either. BEpusdt provenance is documented in its sample README.
This is a source archive, not a maintained SDK or a deployment package.

## Copied BEpusdt documents

`BEpusdt/API_docs/api.md`, `notify.md` and `trade-type.md` are historical provider
documents. They contain contradictory callback acknowledgments (`success` versus
`ok`), differing create-order replay descriptions, and links to files that were
not included in this import. Do not infer payment completion from the examples
or browser returns. The maintained adapter's pinned protocol, exact amount checks
and authenticated callback/reconciliation tests determine Carpool behavior.
