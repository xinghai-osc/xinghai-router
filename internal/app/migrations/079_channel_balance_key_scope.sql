-- Each channel key keeps its own last-known upstream balance row.
alter table channel_balances drop constraint if exists channel_balances_pkey;

-- Rows predating per-key scoping carry a null key_id and cannot be part of a
-- composite primary key; they are superseded by per-key rows on the next
-- balance refresh.
delete from channel_balances where key_id is null;

alter table channel_balances add primary key (channel_id, key_id);

create index if not exists idx_channel_balances_channel on channel_balances(channel_id);
