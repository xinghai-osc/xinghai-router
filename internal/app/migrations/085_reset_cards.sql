-- Reset cards let admins distribute quota resets directly to a user's account.
-- Each card is bound to exactly one subscription (user_subscriptions row) and
-- can reset only that subscription. There is no code to type: the card is
-- attached to the account at issue time, the user's subscription card shows how
-- many reset cards are available, and a "reset" action consumes one card to
-- refill that subscription's remaining request/credit counters (the same effect
-- as the admin "reset quotas" action). Cards are created in batches and are
-- single-use.

create table if not exists reset_cards (
  id uuid primary key,
  batch_id uuid not null,
  subscription_id uuid not null references user_subscriptions(id) on delete cascade,
  enabled boolean not null default true,
  expires_at timestamptz,
  note text not null default '',
  used_by bigint,
  used_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists reset_cards_subscription_idx on reset_cards(subscription_id);
create index if not exists reset_cards_batch_idx on reset_cards(batch_id);
create index if not exists reset_cards_available_idx on reset_cards(subscription_id, enabled, used_at, expires_at);
