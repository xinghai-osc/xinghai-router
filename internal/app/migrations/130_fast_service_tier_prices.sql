BEGIN;

UPDATE pricing_rules p
SET dimension_prices = coalesce(p.dimension_prices, '{}'::jsonb) || jsonb_build_object(
  'service_tiers',
  coalesce(p.dimension_prices->'service_tiers', '{}'::jsonb) || jsonb_build_object(
    'fast', jsonb_build_object(
      'input_per_million', p.input_per_million * 2.5,
      'cached_input_per_million', p.cached_input_per_million * 2.5,
      'output_per_million', p.output_per_million * 2.5
    )
  )
), updated_at=now()
WHERE p.enabled
  AND p.model LIKE 'gpt-%'
  AND p.model NOT LIKE 'gpt-image-%'
  AND p.model <> 'gpt-reserve'
  AND EXISTS (SELECT 1 FROM model_catalog_metadata m WHERE m.model=p.model AND m.owned_by='openai')
  AND coalesce(p.dimension_prices, '{}'::jsonb) #> '{service_tiers,fast}' IS NULL;

INSERT INTO pricing_rules(id,model,input_per_million,cached_input_per_million,output_per_million,multiplier,enabled,currency,dimension_prices)
VALUES (gen_random_uuid(),'glm-5.2-fast-preview',8,2,28,1,true,'CNY','{}'::jsonb)
ON CONFLICT(model) DO UPDATE SET
  input_per_million=excluded.input_per_million,
  cached_input_per_million=excluded.cached_input_per_million,
  output_per_million=excluded.output_per_million,
  multiplier=excluded.multiplier,
  enabled=true,
  currency='CNY',
  updated_at=now();

INSERT INTO audit_logs(id,action,actor,entity_type,entity_id,details,request_method,request_path)
VALUES(gen_random_uuid(),'pricing.fast_tiers_configured','operator-cli','pricing_rules','gpt-and-glm-fast',
jsonb_build_object(
 'gpt_fast_multiplier',2.5,
 'gpt_models',(SELECT jsonb_agg(model ORDER BY model) FROM pricing_rules WHERE enabled AND model LIKE 'gpt-%' AND model NOT LIKE 'gpt-image-%' AND model <> 'gpt-reserve' AND dimension_prices #> '{service_tiers,fast}' IS NOT NULL),
 'glm_model','glm-5.2-fast-preview',
 'glm_prices',jsonb_build_object('input_per_million',8,'cached_input_per_million',2,'output_per_million',28,'currency','CNY')
),'CLI','pricing/fast-tier');

COMMIT;
