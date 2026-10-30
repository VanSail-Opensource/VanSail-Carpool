-- Restore members who already accepted an agreement before a later publication.
-- Keep the original acceptance evidence; first-time onboarding remains gated.
UPDATE users SET onboarding_state='complete'
WHERE onboarding_state='agreement'
  AND (policy_accepted_at IS NOT NULL OR accepted_agreement_revision>0);
