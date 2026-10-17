import type { ResourceMetadata } from '@stackplore/api-client';
import {
  hasExactKeys,
  isDisplayText,
  isRecord,
  isSafeRangeInteger,
  isTimestamp,
} from './product-api-common';

const maximumInteger = Number.MAX_SAFE_INTEGER;
const publicToken = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/u;
const imagePattern = /^[A-Za-z0-9][A-Za-z0-9._:/-]{0,255}(?:@sha256:[a-f0-9]{64})?$/u;
const containerName = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/u;
const containerReasons = new Set([
  'kubernetes.container.waiting',
  'kubernetes.container.running',
  'kubernetes.container.terminated',
  'kubernetes.container.crash_loop',
  'kubernetes.container.image_pull',
  'kubernetes.container.config_error',
  'kubernetes.container.creating',
  'kubernetes.container.out_of_memory',
  'kubernetes.container.completed',
  'kubernetes.container.error',
]);
const conditionTypes = new Set([
  'ready',
  'containers_ready',
  'initialized',
  'scheduled',
  'available',
  'progressing',
  'replica_failure',
  'complete',
  'failed',
]);
const conditionReasons = new Set([
  'kubernetes.condition.true',
  'kubernetes.condition.false',
  'kubernetes.condition.unknown',
]);
const inSet = (value: unknown, choices: ReadonlySet<string>) =>
  typeof value === 'string' && choices.has(value);
const inList = (value: unknown, choices: readonly string[]) =>
  typeof value === 'string' && choices.includes(value);
const optionalQuantity = (value: unknown, maximum: number, positive = false) =>
  value === undefined ||
  (typeof value === 'number' &&
    Number.isFinite(value) &&
    value >= 0 &&
    (!positive || value > 0) &&
    value <= maximum);
const optionalInteger = (value: unknown, minimum: number) =>
  value === undefined || isSafeRangeInteger(value, minimum, maximumInteger);
const boundedArray = (
  value: unknown,
  maximum: number,
  valid: (item: unknown) => boolean,
): value is unknown[] => Array.isArray(value) && value.length <= maximum && value.every(valid);
const unique = (values: readonly unknown[], key: (value: unknown) => string) =>
  new Set(values.map(key)).size === values.length;

function isPort(value: unknown) {
  return (
    isRecord(value) &&
    hasExactKeys(value, ['container_port', 'protocol', 'published'], ['host_port']) &&
    isSafeRangeInteger(value.container_port, 1, 65535) &&
    inList(value.protocol, ['tcp', 'udp', 'sctp']) &&
    typeof value.published === 'boolean' &&
    (value.published
      ? isSafeRangeInteger(value.host_port, 1, 65535)
      : value.host_port === undefined)
  );
}

function isMount(value: unknown) {
  return (
    isRecord(value) &&
    hasExactKeys(value, ['type', 'target', 'read_only']) &&
    inList(value.type, ['volume', 'bind', 'tmpfs']) &&
    isDisplayText(value.target) &&
    value.target.startsWith('/') &&
    typeof value.read_only === 'boolean'
  );
}

function isLabel(value: unknown) {
  if (
    !isRecord(value) ||
    !hasExactKeys(value, ['key', 'value']) ||
    typeof value.value !== 'string' ||
    value.value.length > 256
  )
    return false;
  if (value.key === 'com.docker.compose.project' || value.key === 'com.docker.compose.service')
    return publicToken.test(value.value);
  if (value.key !== 'stackplore.io/depends-on') return false;
  const parts = value.value.split(',');
  return parts.length <= 32 && parts.every((part) => publicToken.test(part));
}

function isDocker(value: unknown) {
  if (
    !isRecord(value) ||
    !hasExactKeys(
      value,
      ['state', 'health', 'restart_count', 'ports', 'mounts', 'labels'],
      [
        'engine_version',
        'image',
        'started_at',
        'exit_code',
        'cpu_limit_cores',
        'memory_limit_bytes',
      ],
    )
  )
    return false;
  return (
    inList(value.state, [
      'created',
      'running',
      'paused',
      'restarting',
      'exited',
      'removing',
      'dead',
      'unknown',
    ]) &&
    inList(value.health, ['healthy', 'unhealthy', 'starting', 'none', 'unknown']) &&
    isSafeRangeInteger(value.restart_count, 0, maximumInteger) &&
    (value.engine_version === undefined ||
      (isDisplayText(value.engine_version) && value.engine_version.length <= 64)) &&
    (value.image === undefined ||
      (typeof value.image === 'string' &&
        value.image.length <= 256 &&
        imagePattern.test(value.image))) &&
    (value.started_at === undefined ||
      (isTimestamp(value.started_at) && !/\.[0-9]{7,}Z$/u.test(value.started_at))) &&
    (value.exit_code === undefined ||
      isSafeRangeInteger(value.exit_code, -2147483648, 2147483647)) &&
    optionalQuantity(value.cpu_limit_cores, 1048576, true) &&
    optionalInteger(value.memory_limit_bytes, 1) &&
    boundedArray(value.ports, 64, isPort) &&
    unique(value.ports, (port) =>
      isRecord(port)
        ? `${String(port.container_port)}:${String(port.host_port)}:${String(port.protocol)}:${String(port.published)}`
        : '',
    ) &&
    boundedArray(value.mounts, 64, isMount) &&
    boundedArray(value.labels, 16, isLabel) &&
    unique(value.labels, (label) => (isRecord(label) ? String(label.key) : ''))
  );
}

function isContainer(value: unknown) {
  return (
    isRecord(value) &&
    hasExactKeys(
      value,
      ['name', 'state', 'ready', 'restart_count'],
      [
        'reason_code',
        'cpu_request_cores',
        'cpu_limit_cores',
        'memory_request_bytes',
        'memory_limit_bytes',
      ],
    ) &&
    typeof value.name === 'string' &&
    containerName.test(value.name) &&
    inList(value.state, ['unknown', 'waiting', 'running', 'terminated']) &&
    typeof value.ready === 'boolean' &&
    isSafeRangeInteger(value.restart_count, 0, maximumInteger) &&
    (value.reason_code === undefined || inSet(value.reason_code, containerReasons)) &&
    optionalQuantity(value.cpu_request_cores, 1e9) &&
    optionalQuantity(value.cpu_limit_cores, 1e9) &&
    optionalInteger(value.memory_request_bytes, 0) &&
    optionalInteger(value.memory_limit_bytes, 0)
  );
}

function isCondition(value: unknown) {
  return (
    isRecord(value) &&
    hasExactKeys(value, ['type', 'status'], ['reason_code']) &&
    inSet(value.type, conditionTypes) &&
    inList(value.status, ['true', 'false', 'unknown']) &&
    (value.reason_code === undefined || inSet(value.reason_code, conditionReasons))
  );
}

function isKubernetes(value: unknown) {
  return (
    isRecord(value) &&
    hasExactKeys(value, ['containers', 'conditions']) &&
    boundedArray(value.containers, 64, isContainer) &&
    unique(value.containers, (item) => (isRecord(item) ? String(item.name) : '')) &&
    boundedArray(value.conditions, 16, isCondition) &&
    unique(value.conditions, (item) => (isRecord(item) ? String(item.type) : ''))
  );
}

export function isResourceMetadata(value: unknown, provider: unknown): value is ResourceMetadata {
  if (!isRecord(value)) return false;
  if (provider === 'docker') return hasExactKeys(value, ['docker']) && isDocker(value.docker);
  if (provider === 'kubernetes')
    return hasExactKeys(value, ['kubernetes']) && isKubernetes(value.kubernetes);
  return false;
}
