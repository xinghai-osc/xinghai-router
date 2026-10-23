ALTER TABLE pricing_rules
  DROP CONSTRAINT IF EXISTS pricing_rules_multiplier_check;

ALTER TABLE pricing_rules
  ADD CONSTRAINT pricing_rules_multiplier_check
  CHECK (multiplier >= 0);
