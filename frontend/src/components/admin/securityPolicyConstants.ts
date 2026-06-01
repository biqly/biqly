import type { SelectOption } from '../ui/Select'

export const SECURITY_POLICY_ROLES = ['viewer', 'analyst', 'developer', 'admin', 'super_admin'] as const

export const FILTER_OPERATOR_OPTIONS: SelectOption[] = [
  { value: 'eq', label: 'Equals (=)' },
  { value: 'neq', label: 'Not Equals (≠)' },
  { value: 'gt', label: 'Greater Than (>)' },
  { value: 'gte', label: 'Greater Than or Equal (≥)' },
  { value: 'lt', label: 'Less Than (<)' },
  { value: 'lte', label: 'Less Than or Equal (≤)' },
  { value: 'in', label: 'In (List)' },
  { value: 'not_in', label: 'Not In (List)' },
  { value: 'is_null', label: 'Is Null' },
  { value: 'is_not_null', label: 'Is Not Null' },
]

export function fieldSelectOptions(fields: string[]): SelectOption[] {
  return fields.map((f) => ({ value: f, label: f }))
}
