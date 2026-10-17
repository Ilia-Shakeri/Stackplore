import type {
  DockerResourceMetadata,
  KubernetesResourceMetadata,
  ProductResource,
} from '@stackplore/api-client';
import { TechnicalText } from '@stackplore/ui';
import type { ReactNode } from 'react';
import { useLocale, type MessageKey } from '../../i18n';
import './resource-metadata.css';

const stateKeys = {
  created: 'metadata.state.created',
  running: 'metadata.state.running',
  paused: 'metadata.state.paused',
  restarting: 'metadata.state.restarting',
  exited: 'metadata.state.exited',
  removing: 'metadata.state.removing',
  dead: 'metadata.state.dead',
  unknown: 'metadata.unknown',
  waiting: 'metadata.state.waiting',
  terminated: 'metadata.state.terminated',
} as const satisfies Record<string, MessageKey>;
const conditionKeys = {
  ready: 'metadata.condition.ready',
  containers_ready: 'metadata.condition.containersReady',
  initialized: 'metadata.condition.initialized',
  scheduled: 'metadata.condition.scheduled',
  available: 'metadata.condition.available',
  progressing: 'metadata.condition.progressing',
  replica_failure: 'metadata.condition.replicaFailure',
  complete: 'metadata.condition.complete',
  failed: 'metadata.condition.failed',
} as const satisfies Record<string, MessageKey>;
const reasonKeys = {
  'kubernetes.container.waiting': 'metadata.state.waiting',
  'kubernetes.container.running': 'metadata.state.running',
  'kubernetes.container.terminated': 'metadata.state.terminated',
  'kubernetes.container.crash_loop': 'metadata.reason.crashLoop',
  'kubernetes.container.image_pull': 'metadata.reason.imagePull',
  'kubernetes.container.config_error': 'metadata.reason.configError',
  'kubernetes.container.creating': 'metadata.reason.creating',
  'kubernetes.container.out_of_memory': 'metadata.reason.outOfMemory',
  'kubernetes.container.completed': 'metadata.condition.complete',
  'kubernetes.container.error': 'metadata.reason.error',
} as const satisfies Record<string, MessageKey>;

export function ResourceMetadataPanel({ resource }: { readonly resource: ProductResource }) {
  const { t } = useLocale();
  return (
    <section className="resource-metadata" aria-labelledby="resource-configuration-title">
      <h2 id="resource-configuration-title">{t('metadata.title')}</h2>
      <p>{t('metadata.description')}</p>
      {resource.metadata === undefined ? (
        <p>{t('metadata.unavailable')}</p>
      ) : 'docker' in resource.metadata ? (
        <DockerFacts
          metadata={resource.metadata.docker}
          container={resource.kind === 'docker.container'}
        />
      ) : (
        <KubernetesFacts metadata={resource.metadata.kubernetes} resource={resource} />
      )}
    </section>
  );
}

function Fact({ label, children }: { readonly label: ReactNode; readonly children: ReactNode }) {
  return (
    <div>
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  );
}

function DockerFacts({
  metadata,
  container,
}: {
  readonly metadata: DockerResourceMetadata;
  readonly container: boolean;
}) {
  const { t, formatNumber, formatDate } = useLocale();
  const unknown = t('metadata.unknown');
  if (!container)
    return (
      <dl className="resource-metadata__facts">
        <Fact label={t('metadata.engine')}>
          <TechnicalText>{metadata.engine_version ?? unknown}</TechnicalText>
        </Fact>
      </dl>
    );
  const healthKeys = {
    healthy: 'product.health.healthy',
    unhealthy: 'product.health.unhealthy',
    starting: 'metadata.health.starting',
    none: 'metadata.health.none',
    unknown: 'metadata.unknown',
  } as const;
  const mountKeys = {
    volume: 'metadata.mount.volume',
    bind: 'metadata.mount.bind',
    tmpfs: 'metadata.mount.tmpfs',
  } as const;
  return (
    <>
      <dl className="resource-metadata__facts">
        <Fact label={t('metadata.state')}>{t(stateKeys[metadata.state])}</Fact>
        <Fact label={t('metadata.health')}>{t(healthKeys[metadata.health])}</Fact>
        <Fact label={t('metadata.restarts')}>{formatNumber(metadata.restart_count)}</Fact>
        <Fact label={t('metadata.engine')}>
          <TechnicalText>{metadata.engine_version ?? unknown}</TechnicalText>
        </Fact>
        <Fact label={t('metadata.image')}>
          <TechnicalText>{metadata.image ?? unknown}</TechnicalText>
        </Fact>
        <Fact label={t('metadata.started')}>
          {metadata.started_at === undefined
            ? unknown
            : formatDate(Date.parse(metadata.started_at), {
                dateStyle: 'medium',
                timeStyle: 'medium',
              })}
        </Fact>
        <Fact label={t('metadata.exit')}>
          {metadata.exit_code === undefined ? unknown : formatNumber(metadata.exit_code)}
        </Fact>
        <Fact label={t('metadata.cpuLimit')}>
          {metadata.cpu_limit_cores === undefined
            ? unknown
            : t('metadata.cores', {
                value: formatNumber(metadata.cpu_limit_cores, { maximumFractionDigits: 4 }),
              })}
        </Fact>
        <Fact label={t('metadata.memoryLimit')}>
          {memory(metadata.memory_limit_bytes, formatNumber, t)}
        </Fact>
      </dl>
      <h3>{t('metadata.ports')}</h3>
      {metadata.ports.length === 0 ? (
        <p>{t('metadata.empty')}</p>
      ) : (
        <ul>
          {metadata.ports.map((port) => (
            <li key={`${String(port.container_port)}:${String(port.host_port)}:${port.protocol}`}>
              <TechnicalText>{`${String(port.container_port)}/${port.protocol}${port.host_port === undefined ? '' : ` → ${String(port.host_port)}`}`}</TechnicalText>
              {' · '}
              {t(port.published ? 'metadata.port.published' : 'metadata.port.exposed')}
            </li>
          ))}
        </ul>
      )}
      <h3>{t('metadata.mounts')}</h3>
      {metadata.mounts.length === 0 ? (
        <p>{t('metadata.empty')}</p>
      ) : (
        <ul>
          {metadata.mounts.map((mount, index) => (
            <li key={`${mount.target}:${String(index)}`}>
              <TechnicalText>{mount.target}</TechnicalText>
              {' · '}
              {t(mountKeys[mount.type])}
              {' · '}
              {t(mount.read_only ? 'metadata.mount.readOnly' : 'metadata.mount.readWrite')}
            </li>
          ))}
        </ul>
      )}
      <h3>{t('metadata.labels')}</h3>
      {metadata.labels.length === 0 ? (
        <p>{t('metadata.empty')}</p>
      ) : (
        <dl className="resource-metadata__facts">
          {metadata.labels.map((label) => (
            <Fact key={label.key} label={<TechnicalText>{label.key}</TechnicalText>}>
              <TechnicalText>{label.value}</TechnicalText>
            </Fact>
          ))}
        </dl>
      )}
    </>
  );
}

function KubernetesFacts({
  metadata,
  resource,
}: {
  readonly metadata: KubernetesResourceMetadata;
  readonly resource: ProductResource;
}) {
  const { t, formatNumber } = useLocale();
  const unknown = t('metadata.unknown');
  return (
    <>
      <h3>{t('metadata.containers')}</h3>
      {metadata.containers.length === 0 ? (
        <p>{t('metadata.empty')}</p>
      ) : (
        <ul className="resource-metadata__containers">
          {metadata.containers.map((container) => {
            const logQuery = new URLSearchParams({
              source: resource.source_id,
              resource_id: resource.id,
              container: container.name,
            });
            return (
              <li key={container.name}>
                <h4>
                  <TechnicalText>{container.name}</TechnicalText>
                </h4>
                <dl className="resource-metadata__facts">
                  <Fact label={t('metadata.state')}>{t(stateKeys[container.state])}</Fact>
                  <Fact label={t('metadata.ready')}>
                    {container.state === 'unknown'
                      ? unknown
                      : t(container.ready ? 'metadata.yes' : 'metadata.no')}
                  </Fact>
                  <Fact label={t('metadata.restarts')}>
                    {container.state === 'unknown'
                      ? unknown
                      : formatNumber(container.restart_count)}
                  </Fact>
                  <Fact label={t('metadata.reason')}>
                    {container.reason_code === undefined
                      ? unknown
                      : t(reasonKeys[container.reason_code])}
                  </Fact>
                  <Fact label={t('metadata.cpuRequest')}>
                    {container.cpu_request_cores === undefined
                      ? unknown
                      : t('metadata.cores', {
                          value: formatNumber(container.cpu_request_cores, {
                            maximumFractionDigits: 4,
                          }),
                        })}
                  </Fact>
                  <Fact label={t('metadata.cpuLimit')}>
                    {container.cpu_limit_cores === undefined
                      ? unknown
                      : t('metadata.cores', {
                          value: formatNumber(container.cpu_limit_cores, {
                            maximumFractionDigits: 4,
                          }),
                        })}
                  </Fact>
                  <Fact label={t('metadata.memoryRequest')}>
                    {memory(container.memory_request_bytes, formatNumber, t)}
                  </Fact>
                  <Fact label={t('metadata.memoryLimit')}>
                    {memory(container.memory_limit_bytes, formatNumber, t)}
                  </Fact>
                </dl>
                {resource.kind === 'kubernetes.pod' || resource.kind === 'kubernetes.container' ? (
                  <nav aria-label={t('metadata.containerLogs', { name: container.name })}>
                    <a href={`/logs?${logQuery.toString()}`}>{t('metadata.logs.current')}</a>
                    {container.state !== 'unknown' && container.restart_count > 0 ? (
                      <a href={`/logs?${logQuery.toString()}&previous=true`}>
                        {t('metadata.logs.previous')}
                      </a>
                    ) : null}
                  </nav>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
      <h3>{t('metadata.conditions')}</h3>
      {metadata.conditions.length === 0 ? (
        <p>{t('metadata.empty')}</p>
      ) : (
        <dl className="resource-metadata__facts">
          {metadata.conditions.map((condition) => (
            <Fact key={condition.type} label={t(conditionKeys[condition.type])}>
              {t(
                condition.status === 'true'
                  ? 'metadata.yes'
                  : condition.status === 'false'
                    ? 'metadata.no'
                    : 'metadata.unknown',
              )}
            </Fact>
          ))}
        </dl>
      )}
    </>
  );
}

function memory(
  value: number | undefined,
  formatNumber: ReturnType<typeof useLocale>['formatNumber'],
  t: ReturnType<typeof useLocale>['t'],
) {
  return value === undefined
    ? t('metadata.unknown')
    : t('metadata.bytes', { value: formatNumber(value) });
}
