alter table channels add column if not exists key_type text not null default 'single' check (key_type in ('single','multi'));

update channels
set key_type = case
    when (select count(*) from channel_api_keys ak where ak.channel_id = channels.id and ak.enabled) > 1 then 'multi'
    else 'single'
end;
