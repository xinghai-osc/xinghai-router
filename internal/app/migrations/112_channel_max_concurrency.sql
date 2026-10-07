-- Channel concurrency limit: the maximum number of simultaneous in-flight
-- upstream requests a single channel may serve. Null or 0 means unlimited.
alter table channels add column if not exists max_concurrency int;
