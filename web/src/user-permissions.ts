type UserAccess = {
  id: string
  role: string
  permissions: string[]
}

export function canManageUser(actor: UserAccess | null, target: UserAccess): boolean {
  if (!actor) return false
  if (actor.role === 'admin') return true
  return actor.permissions.includes('users.manage') && target.role === 'user' && target.permissions.length === 0
}

export function canAuthorizeUser(actor: UserAccess | null, target?: UserAccess | null): boolean {
  return actor?.role === 'admin' && (!target || actor.id !== target.id)
}
