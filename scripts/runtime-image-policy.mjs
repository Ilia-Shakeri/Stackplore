export const POSTGRES_IMAGE_POLICY_PATH = 'deploy/postgres-image.json';
export const MINIMUM_POSTGRES_VERSION = '17.11';
export const GOSU_BUILD_IMAGE =
  'golang:1.26.9-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade';

const policyKeys = [
  'schema_version',
  'image',
  'version',
  'server_version_num',
  'platform',
  'minimum_native_tools',
  'runtime_image',
  'gosu_version',
  'gosu_source_commit',
  'gosu_source_sha256',
  'gosu_build_go',
];

const fail = (message) => {
  throw new Error(`PostgreSQL image policy failed: ${message}`);
};

export function validatePostgresImagePolicy(policy) {
  if (!policy || typeof policy !== 'object' || Array.isArray(policy)) {
    fail('policy must be an object');
  }
  if (
    Object.keys(policy).length !== policyKeys.length ||
    policyKeys.some((key) => !Object.hasOwn(policy, key))
  ) {
    fail('policy fields must match schema 1');
  }
  if (policy.schema_version !== 1) fail('schema_version must be 1');
  const versionMatch =
    typeof policy.version === 'string' ? /^17\.(0|[1-9]\d{0,3})$/u.exec(policy.version) : null;
  if (!versionMatch || versionMatch[0] !== policy.version || Number(versionMatch[1]) < 11) {
    fail(`version must be PostgreSQL 17 at or above ${MINIMUM_POSTGRES_VERSION}`);
  }
  if (policy.server_version_num !== 170_000 + Number(versionMatch[1])) {
    fail('server_version_num must match version');
  }
  if (typeof policy.image !== 'string') fail('image must be a string');
  const imagePrefix = `postgres:${policy.version}-alpine@sha256:`;
  const digest = policy.image.slice(imagePrefix.length);
  if (
    !policy.image.startsWith(imagePrefix) ||
    digest.length !== 64 ||
    !/^[a-f0-9]{64}$/u.test(digest)
  ) {
    fail('image must match version and include a SHA-256 digest');
  }
  if (policy.platform !== 'linux/amd64') fail('platform must be linux/amd64');
  if (policy.minimum_native_tools !== MINIMUM_POSTGRES_VERSION) {
    fail(`minimum_native_tools must be ${MINIMUM_POSTGRES_VERSION}`);
  }
  if (
    typeof policy.runtime_image !== 'string' ||
    !/^stackplore-postgres:(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)$/u.test(
      policy.runtime_image,
    )
  )
    fail('runtime_image must be a versioned Stackplore PostgreSQL image');
  if (policy.gosu_version !== '1.19' || policy.gosu_build_go !== '1.26.9')
    fail('gosu must be version 1.19 rebuilt with Go 1.26.9');
  if (
    typeof policy.gosu_source_commit !== 'string' ||
    !/^[a-f0-9]{40}$/u.test(policy.gosu_source_commit) ||
    typeof policy.gosu_source_sha256 !== 'string' ||
    !/^[a-f0-9]{64}$/u.test(policy.gosu_source_sha256)
  )
    fail('gosu source must include an exact commit and SHA-256 archive digest');
  return policy;
}
