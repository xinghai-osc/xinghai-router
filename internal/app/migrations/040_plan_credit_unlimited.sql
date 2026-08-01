-- credit_amount NULL means unlimited bundled credit; 0 means no credit.
alter table subscription_plans alter column credit_amount drop not null;
