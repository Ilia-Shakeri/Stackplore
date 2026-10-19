import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import process from 'node:process';

const chart = 'deploy/helm/stackplore';
const render = (name, override) => {
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
    ],
    { encoding: 'utf8', timeout: 30_000, maxBuffer: 4 * 1024 * 1024, windowsHide: true },
  );
  assert.equal(result.status, 0, 'Chart render must succeed with a valid release name');
  const documents = result.stdout.split(/^---\s*$/mu).filter((text) => /^kind:/mu.test(text));
  const object = (kind) => {
    const matches = documents.filter((text) => new RegExp(`^kind: ${kind}$`, 'mu').test(text));
    assert.equal(matches.length, 1, `Expected exactly one ${kind}`);
    const text = matches[0];
    const value = /^ {2}name: (.+)$/mu.exec(text)?.[1];
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
    /^ {16}name: (.+)$/mu.exec(ingress.text)?.[1],
    service.name,
    'Ingress must target the named Service',
  );
  assert.equal(/^ {6}serviceAccountName: (.+)$/mu.exec(deployment.text)?.[1], account.name);
  assert.equal(/^ {12}claimName: (.+)$/mu.exec(deployment.text)?.[1], claim.name);
  assert.equal(
    claim.name,
    `${account.name}-runtime`,
    'Existing runtime volume identity must be preserved',
  );
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
process.stdout.write('Nine chart naming, reference and existing-volume cases pass.\n');
