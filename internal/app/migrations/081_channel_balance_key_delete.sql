alter table channel_balances drop constraint if exists channel_balances_key_id_fkey;
alter table channel_balances add constraint channel_balances_key_id_fkey foreign key (key_id) references channel_api_keys(id) on delete cascade;
