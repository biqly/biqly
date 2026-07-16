// Join column type compatibility, shared by the Modeling manual-relationship
// flow and the Query Builder JOIN DATA editor. Mirrors the backend groups in
// internal/query/join_compat.go.
//
// Groups: integers, decimals (numerics), text-likes, uuid, boolean,
// timestamp, date, json. Cross-group pairs that remain SQL-joinable:
// integers<->decimals and date<->timestamp. json only joins json; uuid only
// joins uuid. Unknown types fail open (treated as compatible) so exotic
// dialect types never block a selection.

const JOIN_TYPE_GROUP_BY_RAW_TYPE: Record<string, string> = {
  // integers (incl. serial pseudo-types)
  smallint: 'integer',
  int2: 'integer',
  integer: 'integer',
  int: 'integer',
  int4: 'integer',
  bigint: 'integer',
  int8: 'integer',
  serial: 'integer',
  serial4: 'integer',
  bigserial: 'integer',
  serial8: 'integer',
  // arbitrary-precision / floating numerics
  numeric: 'decimal',
  decimal: 'decimal',
  'double precision': 'decimal',
  float: 'decimal',
  float4: 'decimal',
  float8: 'decimal',
  real: 'decimal',
  money: 'decimal',
  // text-likes
  text: 'text',
  'character varying': 'text',
  varchar: 'text',
  character: 'text',
  char: 'text',
  citext: 'text',
  nvarchar: 'text',
  nchar: 'text',
  string: 'text',
  // uuid is its own group: not implicitly joinable to text
  uuid: 'uuid',
  uniqueidentifier: 'uuid',
  // booleans
  boolean: 'boolean',
  bool: 'boolean',
  // timestamps
  timestamp: 'timestamp',
  'timestamp without time zone': 'timestamp',
  'timestamp with time zone': 'timestamp',
  timestamptz: 'timestamp',
  datetime: 'timestamp',
  // dates
  date: 'date',
  // json is only joinable to itself
  json: 'json',
  jsonb: 'json',
}

/**
 * Maps a raw column data type to its join compatibility group, or '' when the
 * type is unknown (callers must fail open).
 */
export function normalizeJoinDataType(dataType: string): string {
  const type = dataType
    .toLowerCase()
    .replace(/\(.+\)/, '')
    .replace(/\s+/g, ' ')
    .trim()
  return JOIN_TYPE_GROUP_BY_RAW_TYPE[type] ?? ''
}

const CROSS_GROUP_PAIRS = new Set(['integer|decimal', 'date|timestamp'])

/**
 * Whether two raw column data types are SQL-joinable. Unknown types fail
 * open (compatible).
 */
export function joinDataTypesCompatible(left: string, right: string): boolean {
  const l = normalizeJoinDataType(left)
  const r = normalizeJoinDataType(right)
  if (!l || !r) {
    return true
  }
  if (l === r) {
    return true
  }
  return CROSS_GROUP_PAIRS.has(`${l}|${r}`) || CROSS_GROUP_PAIRS.has(`${r}|${l}`)
}
