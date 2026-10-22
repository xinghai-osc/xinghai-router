import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const dependencyRequire = createRequire(new URL('../node_modules/.pnpm/node_modules/security-audit.cjs', import.meta.url))
const yaml = dependencyRequire('yaml')
const workspace = yaml.parse(readFileSync(new URL('../pnpm-workspace.yaml', import.meta.url), 'utf8'))
const documents = yaml.parseAllDocuments(readFileSync(new URL('../pnpm-lock.yaml', import.meta.url), 'utf8'))
for (const document of documents) assert.equal(document.errors.length, 0)
const lock = documents.map(document => document.toJS()).find(document => document?.patchedDependencies)
assert.ok(lock, 'Locked dependency patches are required')

const mitigations = {
  'GHSA-86w9-cpqp-85rv': {
    name: 'node-forge', version: '1.4.0',
    hash: 'b6876574a5de19521a766856b293aeae2689848c5d8dab03f93040be0a64b8b4',
  },
  'GHSA-vfj7-8cjw-p6xm': {
    name: 'braces', version: '3.0.3',
    hash: '523afce3b412ced1d821c7ff74be8af5b233ec7f7964bcfcf3d45cfa55539365',
  },
}

for (const { name, version, hash } of Object.values(mitigations)) {
  const id = `${name}@${version}`
  const path = `patches/${id}.patch`
  assert.equal(workspace.patchedDependencies?.[id], path)
  assert.equal(lock.patchedDependencies[id], hash)
  assert.equal(createHash('sha256').update(readFileSync(new URL(`../${path}`, import.meta.url))).digest('hex'), hash)
  assert.equal(dependencyRequire(`${name}/package.json`).version, version)
}

const tests = spawnSync(process.execPath, ['--test', 'tests/dependency-security.test.mjs'], { cwd: root, stdio: 'inherit' })
assert.equal(tests.status, 0, 'Dependency attack regressions must pass')

const audit = spawnSync('pnpm', ['audit', '--json', '--registry=https://registry.npmjs.org'], {
  cwd: root, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024,
})
if (audit.error) throw audit.error
assert.ok(audit.status === 0 || audit.status === 1, `Audit failed: ${audit.stderr}`)
const report = JSON.parse(audit.stdout)
process.stdout.write(`${JSON.stringify(report, null, 2)}\n`)
assert.ok(report.metadata?.vulnerabilities && report.advisories, 'Audit must return complete vulnerability results')
const findings = Object.values(report.advisories)
assert.equal(Object.values(report.metadata.vulnerabilities).reduce((sum, count) => sum + count, 0), findings.length)
for (const advisory of findings) {
  const mitigation = mitigations[advisory.github_advisory_id]
  assert.ok(mitigation, `Unmitigated advisory: ${advisory.github_advisory_id} (${advisory.module_name})`)
  assert.equal(advisory.module_name, mitigation.name)
  assert.ok(advisory.findings.length > 0)
  for (const finding of advisory.findings) assert.equal(finding.version, mitigation.version)
  process.stdout.write(`LOCALLY MITIGATED, still reported by registry: ${advisory.github_advisory_id} ${mitigation.name}@${mitigation.version}; patch hash and attack regressions verified.\n`)
}
process.stdout.write(`Audit reviewed ${findings.length} advisories; no unmitigated advisories remain.\n`)
