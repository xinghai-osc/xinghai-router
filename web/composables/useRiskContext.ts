import { endpoints, type RiskPurpose } from '~/src/api'
import { collectRiskProbe } from '~/src/risk-context'

export function useRiskContext() {
  async function prepare(purpose: RiskPurpose): Promise<string | undefined> {
    if (!import.meta.client) return undefined
    const settings = await endpoints.getSiteSettings()
    const evidence = await collectRiskProbe(settings.risk_probe)
    const context = await endpoints.createRiskContext({ purpose, ...evidence })
    return context.id
  }

  return { prepare }
}
