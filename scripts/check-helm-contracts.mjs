import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, rmdirSync, unlinkSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import process from 'node:process';

const directory = mkdtempSync(join(tmpdir(), 'stackplore-chart-types-'));
const validator = join(directory, process.platform === 'win32' ? 'validate.exe' : 'validate');
process.on('exit', () => {
  if (existsSync(validator)) unlinkSync(validator);
  rmdirSync(directory);
});
const build = spawnSync(
  'go',
  ['-C', 'apps/api', 'build', '-o', validator, '../../scripts/check-helm-types.go'],
  {
    encoding: 'utf8',
    timeout: 180_000,
    maxBuffer: 1024 * 1024,
    windowsHide: true,
  },
);
assert.equal(build.status, 0, `Pinned Kubernetes chart validator build failed: ${build.stderr}`);
const scalar = (value) => (value?.startsWith('"') ? JSON.parse(value) : value);
let cases = 0;
const chart = 'deploy/helm/stackplore';
const render = (name, override, extra = []) => {
  const result = spawnSync(
    'helm',
    [
      'template',
      name,
      chart,
      '--set',
      'ingress.enabled=true',
      '--set',
      'api.kubernetes.enabled=true',
      '--set',
      'api.kubernetes.namespaces[0]=default',
      ...(override === undefined ? [] : ['--set-string', `fullnameOverride=${override}`]),
      ...extra,
    ],
    { encoding: 'utf8', timeout: 30_000, maxBuffer: 4 * 1024 * 1024, windowsHide: true },
  );
  assert.equal(result.status, 0, 'Chart render must succeed with a valid release name');
  const namespaceIndex = extra.indexOf('--namespace');
  const typed = spawnSync(
    validator,
    namespaceIndex < 0 ? [] : ['--namespace', extra[namespaceIndex + 1]],
    {
      input: result.stdout,
      encoding: 'utf8',
      timeout: 10_000,
      maxBuffer: 1024 * 1024,
      windowsHide: true,
    },
  );
  assert.equal(typed.status, 0, `Rendered Kubernetes types must decode: ${typed.stderr}`);
  const documents = result.stdout.split(/^---\s*$/mu).filter((text) => /^kind:/mu.test(text));
  const object = (kind) => {
    const matches = documents.filter((text) => new RegExp(`^kind: ${kind}$`, 'mu').test(text));
    assert.equal(matches.length, 1, `Expected exactly one ${kind}`);
    const text = matches[0];
    const value = scalar(/^ {2}name: (.+)$/mu.exec(text)?.[1]);
    assert.ok(value, `${kind} needs a metadata name`);
    return { name: value, text };
  };
  const service = object('Service');
  const deployment = object('Deployment');
  const ingress = object('Ingress');
  const account = object('ServiceAccount');
  const claim = object('PersistentVolumeClaim');
  assert.ok(service.name.length <= 63, 'Service name exceeds the DNS label budget');
  assert.match(service.name, /^[a-z][a-z0-9-]*[a-z0-9]$/u);
  assert.equal(service.name, deployment.name, 'Application names must agree');
  assert.equal(
    scalar(/^ {16}name: (.+)$/mu.exec(ingress.text)?.[1]),
    service.name,
    'Ingress must target the named Service',
  );
  assert.equal(scalar(/^ {6}serviceAccountName: (.+)$/mu.exec(deployment.text)?.[1]), account.name);
  assert.equal(scalar(/^ {12}claimName: (.+)$/mu.exec(deployment.text)?.[1]), claim.name);
  assert.equal(
    claim.name,
    `${account.name}-runtime`,
    'Existing runtime volume identity must be preserved',
  );
  cases++;
  return service.name;
};

assert.equal(render('normal'), 'normal-stackplore-api');
assert.equal(render('123'), 'sp-123-stackplore-api');
assert.equal(render('normal', '123'), 'sp-123-api');
// A previously valid maximum-length Service keeps its deployed name.
const oldBoundary = 'a'.repeat(48);
assert.equal(render(oldBoundary), `${oldBoundary}-stackplore-api`);
const first = render('stackplore-proof-with-a-long-name-shared-prefix-alpha');
const second = render('stackplore-proof-with-a-long-name-shared-prefix-beta');
assert.notEqual(first, second, 'Long release names must keep distinct Services');
const override = render('normal', 'a'.repeat(63));
assert.match(override, /^[a-z0-9-]+-[a-f0-9]{8}-api$/u);
assert.equal(render('normal', 'a'.repeat(63)), override, 'Name generation must be stable');
assert.match(render('normal', `1${'a'.repeat(62)}`), /^sp-[a-z0-9-]+-[a-f0-9]{8}-api$/u);
for (const value of ['true', 'null', 'yes', '0123', '1e3']) {
  render(value);
  render('normal', value);
}
for (const value of ['123', 'true', 'null'])
  render('normal', undefined, ['--set-string', `nameOverride=${value}`]);
const flags = [
  'api.setup.enabled',
  'api.kubernetes.clusterObservation',
  'api.webhook.enabled',
  'api.matrix.enabled',
  'api.databaseTLS.enabled',
];
const references = [
  'api.existingSecret',
  'api.databaseURLKey',
  'api.databaseTLS.caSecretName',
  'api.databaseTLS.caCertificateKey',
  'api.setup.proofSecretKey',
  'api.setup.expirySecretKey',
  'api.webhook.urlSecretKey',
  'api.webhook.signingKeySecretKey',
  'api.matrix.originSecretKey',
  'api.matrix.roomIdSecretKey',
  'api.matrix.accessTokenSecretKey',
  'ingress.className',
  'ingress.tls.secretName',
];
render('numeric-options', '123', [
  '--namespace',
  '123',
  ...flags.flatMap((key) => ['--set', `${key}=true`]),
  ...references.flatMap((key) => ['--set-string', `${key}=123`]),
]);
process.stdout.write(`${cases} typed chart naming, reference and existing-volume cases pass.\n`);
