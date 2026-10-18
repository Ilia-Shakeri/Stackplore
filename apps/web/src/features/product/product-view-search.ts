const token = /^[A-Za-z][A-Za-z0-9_.:-]{0,252}$/u;
const resource = /^r1_[A-Za-z0-9_-]{43}$/u;
const enums: Readonly<Record<string, readonly string[]>> = {
  provider: ['demo', 'docker', 'kubernetes'],
  health: ['healthy', 'degraded', 'unhealthy', 'unknown'],
  availability: ['available', 'stale', 'unavailable', 'unsupported', 'permission_denied'],
  scope_kind: ['global', 'project', 'namespace'],
  sort: ['name', 'kind', 'observed_at'],
  direction: ['asc', 'desc'],
  previous: ['true', 'false'],
  wrap: ['true', 'false'],
  timestamps: ['true', 'false'],
  tail: ['live', 'paused'],
  minutes: ['60', '360', '1440'],
  severity: ['trace', 'debug', 'info', 'warning', 'error', 'critical', 'unknown'],
  alert_state: ['pending', 'firing', 'acknowledged', 'resolved'],
  topology_health: ['healthy', 'degraded', 'unhealthy', 'unknown'],
  topology_group: ['none', 'environment', 'scope', 'kind', 'provider'],
  topology_neighbors: ['all', 'upstream', 'downstream', 'neighbors'],
};
const textKeys = new Set(['scope_name', 'log_q', 'event_q', 'topology_q', 'topology_scope']);
const tokenKeys = new Set([
  'environment_id',
  'kind',
  'container',
  'topology_kind',
  'topology_environment',
]);
export function validViewParameter(name: string, value: string) {
  if (value === '') return true;
  if (name === 'rule_cursor') return /^[A-Za-z0-9_-]{1,4096}$/u.test(value);
  if (name === 'container') return /^[A-Za-z0-9][A-Za-z0-9_.-]{0,255}$/u.test(value);
  if (name === 'environment_id' || name === 'topology_environment')
    return /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u.test(value);
  if (name === 'kind' || name === 'topology_kind')
    return value.length <= 96 && /^[a-z][a-z0-9_-]*(?:\.[a-z][a-z0-9_-]*)+$/u.test(value);
  if (name === 'scope_name') return value.length <= 256 && !/[\p{Cc}\p{Zl}\p{Zp}]/u.test(value);
  if (name === 'topology_scope') return value.length <= 300 && !/[\p{Cc}\p{Zl}\p{Zp}]/u.test(value);
  if (enums[name] !== undefined) return enums[name].includes(value);
  if (name === 'resource_id' || name === 'topology_selected') return resource.test(value);
  if (tokenKeys.has(name)) return token.test(value);
  if (textKeys.has(name)) return value.length <= 200 && !/[\p{Cc}\p{Zl}\p{Zp}]/u.test(value);
  if (name === 'from' || name === 'to')
    return (
      /^(?!0000)\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,6})?Z$/u.test(value) &&
      Number.isFinite(Date.parse(value))
    );
  return false;
}

export function productViewSearch(search: Record<string, unknown>): Record<string, string> {
  const accepted: Record<string, string> = {};
  for (const [name, value] of Object.entries(search)) {
    if (typeof value === 'string' && value !== '' && validViewParameter(name, value))
      accepted[name] = value;
    if (typeof value === 'boolean' && validViewParameter(name, String(value)))
      accepted[name] = String(value);
    if (typeof value === 'number' && validViewParameter(name, String(value)))
      accepted[name] = String(value);
  }
  return accepted;
}
