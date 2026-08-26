-- Each channel key keeps its own last-known upstream balance row.
alter table channel_balances drop constraint if exists channel_balances_pkey;

alter table channel_balances add primary key (channel_id, key_id);

create index if not exists idx_channel_balances_channel on channel_balances(channel_id);
