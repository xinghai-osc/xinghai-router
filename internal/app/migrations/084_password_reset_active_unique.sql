-- At most one unconsumed password-reset token may exist per email. The request
-- handler consumes old tokens and inserts the new one in a single transaction;
-- this index is the backstop that makes two concurrent requests atomic.
create unique index if not exists password_reset_tokens_active_email_idx
  on password_reset_tokens(email) where consumed_at is null;
