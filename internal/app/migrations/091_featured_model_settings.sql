alter table site_settings
  add column if not exists featured_enabled boolean not null default true,
  add column if not exists featured_model text not null default '',
  add column if not exists featured_copy jsonb not null default '{"zh":{"badge":"推荐模型","title":"探索最新模型","body":"浏览已接入模型，按价格与分组选择适合你的模型。","cta":"查看详情"},"zh-Hant":{"badge":"推薦模型","title":"探索最新模型","body":"瀏覽已接入模型，按價格與分組選擇適合你的模型。","cta":"查看詳情"},"en":{"badge":"Featured model","title":"Explore the latest models","body":"Browse connected models and choose one by price and group.","cta":"View details"}}'::jsonb;
