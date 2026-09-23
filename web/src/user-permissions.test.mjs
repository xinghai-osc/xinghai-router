import assert from 'node:assert/strict'
import test from 'node:test'
import { canAuthorizeUser, canManageUser } from './user-permissions.ts'

const admin = { id: '1', role: 'admin', permissions: [] }
const manager = { id: '2', role: 'operator', permissions: ['users.manage'] }
const ordinary = { id: '3', role: 'user', permissions: [] }

test('user management does not imply authorization', () => {
  assert.equal(canManageUser(manager, ordinary), true)
  assert.equal(canAuthorizeUser(manager, ordinary), false)
  assert.equal(canAuthorizeUser({ ...manager, permissions: ['users.authorize'] }, ordinary), false)
  assert.equal(canManageUser(null, ordinary), false)
  assert.equal(canAuthorizeUser(null, ordinary), false)
})

test('only administrators may authorize other accounts', () => {
  assert.equal(canAuthorizeUser(admin, ordinary), true)
  assert.equal(canAuthorizeUser(admin), true)
  assert.equal(canAuthorizeUser(admin, admin), false)
})

test('profile editing cannot take over privileged accounts', () => {
  assert.equal(canManageUser(manager, admin), false)
  assert.equal(canManageUser(manager, manager), false)
  assert.equal(canManageUser(manager, { ...ordinary, permissions: ['logs.read'] }), false)
  assert.equal(canManageUser(manager, { ...ordinary, role: 'operator' }), false)
  assert.equal(canManageUser(admin, manager), true)
  assert.equal(canManageUser(admin, admin), true)
})
