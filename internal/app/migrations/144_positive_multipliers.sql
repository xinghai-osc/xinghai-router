UPDATE groups SET multiplier = 1 WHERE multiplier <= 0;

ALTER TABLE groups
  DROP CONSTRAINT IF EXISTS groups_multiplier_check;

ALTER TABLE groups
  ADD CONSTRAINT groups_multiplier_check CHECK (multiplier > 0);

-- A zero pricing multiplier used to mean "free". Multipliers must now be
-- positive, so rules that were free are disabled instead of silently starting
-- to bill; re-enable them with an explicit positive multiplier.
UPDATE pricing_rules SET enabled = false WHERE multiplier <= 0;

UPDATE pricing_rules SET multiplier = 1 WHERE multiplier <= 0;

ALTER TABLE pricing_rules
  DROP CONSTRAINT IF EXISTS pricing_rules_multiplier_check;

ALTER TABLE pricing_rules
  ADD CONSTRAINT pricing_rules_multiplier_check CHECK (multiplier > 0);
