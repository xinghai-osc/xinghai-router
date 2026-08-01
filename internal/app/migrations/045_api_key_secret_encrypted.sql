-- Allow the full API key secret to be stored encrypted so it can be revealed later.
-- Legacy keys created before this migration keep secret_encrypted empty and stay
-- reveal-only-at-creation; the reveal endpoint returns an empty key for those rows.
alter table api_keys add column if not exists secret_encrypted text not null default '';