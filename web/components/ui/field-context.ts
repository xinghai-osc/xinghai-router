import type { ComputedRef, InjectionKey } from 'vue'

export interface UiFieldContext {
  id: ComputedRef<string>
  describedBy: ComputedRef<string | undefined>
  invalid: ComputedRef<boolean>
  required: ComputedRef<boolean>
}

export const uiFieldContextKey: InjectionKey<UiFieldContext> = Symbol('ui-field')
