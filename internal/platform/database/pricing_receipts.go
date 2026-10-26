package database

// Ledger event time, rather than mutable purchase totals, defines daily receipts.
// Batch debits and mid-term add-on adjustments resolve through their own references.
const pricingComboReceiptJoins = ` FROM ledger_entries l LEFT JOIN purchases p ON p.id=l.reference_id LEFT JOIN renewal_batches b ON b.id=l.reference_id LEFT JOIN purchases source ON source.id=b.source_purchase_id LEFT JOIN purchase_addon_adjustments a ON a.id=l.reference_id LEFT JOIN purchases added ON added.id=a.purchase_id `
const pricingComboReceiptScope = ` COALESCE(p.combo_id,source.combo_id,added.combo_id)=? AND l.kind IN ('purchase_debit','automatic_renewal','purchase_addon_debit') `
