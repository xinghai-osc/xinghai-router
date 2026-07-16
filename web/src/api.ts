export interface User { id: string; email: string; name: string; role: string; enabled: boolean; balance: number; reserved: number; permissions: string[]; groups: string[]; created_at: string }
export interface ApiKey { id: string; user_id: string; name: string; key_prefix: string; group_id: string; group_name: string; expires_at: string | null; revoked_at: string | null; last_used_at: string | null; created_at: string }
export interface Channel { id: string; name: string; base_url: string; provider: 'openai' | 'ollama' | 'kimi' | 'opencode_go' | 'anthropic'; models: string[]; enabled: boolean; priority: number; groups: string[]; created_at: string }
export interface Group { id: string; name: string; multiplier: number; created_at: string }
export interface RequestLog { request_id: string; user_id: string | null; api_key_id: string | null; channel_id: string | null; model: string; status_code: number; prompt_tokens: number | null; completion_tokens: number | null; total_tokens: number | null; duration_ms: number; error_code: string | null; created_at: string }
export interface Account { id: string; email: string; name: string; role: string; permissions: string[]; balance: number; reserved: number }
export interface Pricing { id: string; model: string; input_per_million: number; cached_input_per_million: number; output_per_million: number; multiplier: number; enabled: boolean; updated_at: string }
export interface CatalogGroup { id: string; name: string; multiplier: number }
export interface CatalogModel { id: string; model: string; input_per_million: number | null; cached_input_per_million: number | null; output_per_million: number | null; multiplier: number | null; groups: CatalogGroup[] }
export interface UsageRecord { request_id: string; model: string; prompt_tokens: number; cached_prompt_tokens: number; completion_tokens: number; cost: number; status: string; created_at: string }
export interface ActivityLog { id: string; type: 'request' | 'login' | 'register' | 'logout' | 'topup' | 'operation'; action: string; user_id: string; user_name: string; model: string; group_id: string; group_name: string; status_code: number | null; duration_ms: number | null; prompt_tokens: number; completion_tokens: number; total_tokens: number; cost: number; details: Record<string, unknown>; created_at: string }
export interface LedgerEntry { id: string; amount: number; balance_after: number; kind: string; request_id: string | null; note: string | null; created_at: string }
export interface ModelRanking { rank: number; previous_rank?: number; model_name: string; vendor: string; total_tokens: number; share: number; growth_pct: number }
export interface VendorRanking { rank: number; vendor: string; total_tokens: number; share: number; growth_pct: number; models_count: number; top_model: string }
export interface RankingMover { model_name: string; vendor: string; rank_delta: number; current_rank: number; growth_pct: number }
export interface Rankings { period: string; models: ModelRanking[]; vendors: VendorRanking[]; top_movers: RankingMover[]; top_droppers: RankingMover[]; total_tokens: number; updated_at: string }

let token = import.meta.client ? sessionStorage.getItem('xinghai.admin-token') ?? '' : ''
export const getToken = () => token
export const setToken = (value: string) => { token = value.trim(); sessionStorage.setItem('xinghai.admin-token', token) }
export const clearToken = () => { token = ''; sessionStorage.removeItem('xinghai.admin-token') }

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api${path}`, { ...init, headers: { Authorization: `Bearer ${token}`, ...(init.body ? { 'Content-Type': 'application/json' } : {}), ...init.headers } })
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    throw new Error(body?.error?.message ?? `请求失败 (${response.status})`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}
