export function joinRelationshipCardinality(rel?: string): string {
  if (!rel) {
    return '1:1'
  }
  switch (rel) {
    case 'many_to_one':
      return 'N:1'
    case 'one_to_many':
      return '1:N'
    case 'one_to_one':
      return '1:1'
    case 'many_to_many':
      return 'N:N'
    default:
      return rel.replace(/_/g, '-')
  }
}
