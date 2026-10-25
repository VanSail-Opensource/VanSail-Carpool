-- Presentation and administrative availability are independent of member
-- preferences and immutable PAYG accounting. Hidden changes presentation only.
ALTER TABLE squad_product_overrides ADD COLUMN payg_state TEXT NOT NULL DEFAULT 'normal'
 CHECK(payg_state IN ('normal','hidden','unavailable'));
