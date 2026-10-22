import assert from 'node:assert/strict'
import { generateKeyPairSync } from 'node:crypto'
import { createRequire } from 'node:module'
import { test } from 'node:test'

const dependencyRequire = createRequire(new URL('../node_modules/.pnpm/node_modules/security-tests.cjs', import.meta.url))
const forge = dependencyRequire('node-forge')
const braces = dependencyRequire('braces')

const digestInfo = (digest, extra = [], withNull = true) => {
  const { asn1 } = forge
  const node = (type, constructed, value) => asn1.create(asn1.Class.UNIVERSAL, type, constructed, value)
  const algorithm = [node(asn1.Type.OID, false, asn1.oidToDer(forge.oids.sha256).getBytes())]
  if (withNull) algorithm.push(node(asn1.Type.NULL, false, ''))
  algorithm.push(...extra)
  return asn1.toDer(node(asn1.Type.SEQUENCE, true, [
    node(asn1.Type.SEQUENCE, true, algorithm),
    node(asn1.Type.OCTETSTRING, false, digest),
  ])).getBytes()
}

test('node-forge rejects extra nested DigestAlgorithm children while accepting normal signatures', () => {
  const pair = generateKeyPairSync('rsa', { modulusLength: 2048, publicExponent: 3 })
  const privateKey = forge.pki.privateKeyFromPem(pair.privateKey.export({ type: 'pkcs1', format: 'pem' }))
  const publicKey = forge.pki.publicKeyFromPem(pair.publicKey.export({ type: 'pkcs1', format: 'pem' }))
  const digest = forge.md.sha256.create().update('fake security regression message').digest().getBytes()
  const { asn1 } = forge
  const garbage = asn1.create(asn1.Class.UNIVERSAL, asn1.Type.OCTETSTRING, false, 'injected-garbage')
  const duplicateNull = asn1.create(asn1.Class.UNIVERSAL, asn1.Type.NULL, false, '')
  const nestedSequence = asn1.create(asn1.Class.UNIVERSAL, asn1.Type.SEQUENCE, true, [garbage])
  for (const withNull of [false, true]) {
    const valid = privateKey.sign(digestInfo(digest, [], withNull), 'NONE')
    assert.equal(publicKey.verify(digest, valid), true)
    for (const extra of [[garbage], [nestedSequence], [duplicateNull, garbage]]) {
      const malformed = privateKey.sign(digestInfo(digest, extra, withNull), 'NONE')
      assert.throws(() => publicKey.verify(digest, malformed), /does not contain a valid RSASSA-PKCS1-v1_5 DigestInfo/)
    }
  }
})

const stringEntrypoints = {
  braces: input => braces(input),
  array: input => braces([input]),
  create: input => braces.create(input),
  parse: input => braces.parse(input),
  compile: input => braces.compile(input),
  expand: input => braces.expand(input),
  stringify: input => braces.stringify(input),
  'create-expand': input => braces.create(input, { expand: true }),
  'disabled-range-limit': input => braces.expand(input, { rangeLimit: false, maxLength: Infinity }),
}

const depthError = error => error instanceof SyntaxError && /maximum nesting depth/.test(error.message)

for (const [name, call] of Object.entries(stringEntrypoints)) {
  test(`braces ${name} rejects deeply nested patterns below its character limit`, () => {
    for (const pattern of [
      '{'.repeat(4000) + 'a,b' + '}'.repeat(4000),
      '('.repeat(4000) + 'x' + ')'.repeat(4000),
      '{('.repeat(2000) + 'a,b' + ')}'.repeat(2000),
      '{'.repeat(4000) + 'x',
    ]) {
      assert.ok(pattern.length < 10000)
      assert.throws(() => call(pattern), depthError)
    }
  })
}

const deepAST = () => {
  const ast = { type: 'root', nodes: [] }
  let parent = ast
  for (let depth = 0; depth < 5000; depth++) {
    const child = { type: 'paren', nodes: [], parent }
    parent.nodes.push(child)
    parent = child
  }
  parent.nodes.push({ type: 'text', value: 'x', parent })
  return ast
}

const astEntrypoints = {
  braces: input => braces(input),
  array: input => braces([input]),
  create: input => braces.create(input),
  compile: input => braces.compile(input),
  expand: input => braces.expand(input),
  stringify: input => braces.stringify(input),
  'create-expand': input => braces.create(input, { expand: true }),
  'direct-compile': dependencyRequire('braces/lib/compile'),
  'direct-expand': dependencyRequire('braces/lib/expand'),
  'direct-stringify': dependencyRequire('braces/lib/stringify'),
}

for (const [name, call] of Object.entries(astEntrypoints)) {
  test(`braces ${name} rejects deeply nested and cyclic supplied ASTs`, () => {
    assert.throws(() => call(deepAST()), depthError)
    const cyclic = { type: 'root', nodes: [] }
    cyclic.nodes.push(cyclic)
    assert.throws(() => call(cyclic), depthError)
  })
}

test('braces preserves ordinary patterns, ASTs, escaped braces, and quoted braces', () => {
  assert.deepEqual(braces.expand('a/{b,c}/d'), ['a/b/d', 'a/c/d'])
  assert.deepEqual(braces.expand('item-{1..3}'), ['item-1', 'item-2', 'item-3'])
  assert.equal(braces.compile('a/{b,c}/d'), 'a/(b|c)/d')
  assert.equal(braces.stringify(braces.parse('a/{b,c}/d')), 'a/{b,c}/d')
  assert.deepEqual(braces.expand(braces.parse('a/{b,c}/d')), ['a/b/d', 'a/c/d'])
  assert.equal(braces.compile('('.repeat(60) + 'x' + ')'.repeat(60)), '('.repeat(60) + 'x' + ')'.repeat(60))
  assert.doesNotThrow(() => braces.parse('\\{'.repeat(4000)))
  assert.doesNotThrow(() => braces.parse('"' + '{'.repeat(4000) + '"'))
})

test('node-forge rejects the upstream PR 1152 nested DigestAlgorithm signature vector', () => {
  const modulus = new forge.jsbn.BigInteger('E932AC92252F585B3A80A4DD76A897C8B7652952FE788F6EC8DD640587A1EE5647670A8AD4C2BE0F9FA6E49C605ADF77B5174230AF7BD50E5D6D6D6D28CCF0A886A514CC72E51D209CC772A52EF419F6A953F3135929588EBE9B351FCA61CED78F346FE00DBB6306E5C2A4C6DFC3779AF85AB417371CF34D8387B9B30AE46D7A5FF5A655B8D8455F1B94AE736989D60A6F2FD5CADBFFBD504C5A756A2E6BB5CECC13BCA7503F6DF8B52ACE5C410997E98809DB4DC30D943DE4E812A47553DCE54844A78E36401D13F77DC650619FED88D8B3926E3D8E319C80C744779AC5D6ABE252896950917476ECE5E8FC27D5F053D6018D91B502C4787558A002B9283DA7', 16)
  const key = forge.pki.rsa.setPublicKey(modulus, new forge.jsbn.BigInteger('3'))
  const signature = forge.util.hexToBytes('a4ae63dd5e7712b78f4870d0f51e294df5503d4f16c5d27ae33370981fb57f0de49f50f3d6a04666774cd984cd13972db9bf8e12bd294ef0ddc916c7c86cbae63efd7b6b97885e69760c208a40f1aecc76a90d7af5145177efce1bb55807a8d05c20b1596753ba710642fc9acdde6c160232654662c77cc4466c8257a38edb49f894e8845d0fd987b857ced88f4b62505a080bd87ef700d35d392a6e8f6fde34250c50b86fae606cb551215e8f4813239b77651d5565ad453698c071d48c31e8e526fb4a37610f64b3e1fb8e5be5898e408ad08197a0947794a530b54f84485377ce4a7488ed485ce4e5e105dd89698a472f390c3b1b76bc16b73276c4d1c81d')
  const digest = forge.md.sha256.create().update('hello world!').digest().getBytes()
  assert.throws(() => key.verify(digest, signature), /does not contain a valid RSASSA-PKCS1-v1_5 DigestInfo/)
})
