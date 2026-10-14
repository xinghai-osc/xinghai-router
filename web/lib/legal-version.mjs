import { createHash } from 'node:crypto'

export function legalVersion(document, dictionaries) {
  const keys = [...new Set([...document.matchAll(/\bsite\.([A-Za-z0-9_]+)/g)].map(match => match[1]))].sort()
  const content = Object.entries(dictionaries).sort(([a], [b]) => a.localeCompare(b)).map(([locale, messages]) => [
    locale, keys.map(key => [key, messages[key] ?? null]),
  ])
  return createHash('sha256').update(JSON.stringify(content)).digest('hex')
}
