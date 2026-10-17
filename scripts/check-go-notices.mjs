import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { lstat, mkdir, readFile, readdir, writeFile } from 'node:fs/promises';
import { dirname, relative, resolve, sep } from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const inventoryDirectory = 'apps/api/licenses/modules';
const manifestPath = `${inventoryDirectory}/manifest.json`;
const commands = ['./cmd/operator', './cmd/server', './cmd/storage'];
const targets = ['linux/amd64', 'windows/amd64'];
const scope =
  'External Go modules linked by the three commands for the listed CGO-disabled targets, plus the pinned Go distribution license for the standard library; excludes tests, JavaScript, fonts and container images.';
const toolchainPath = `${inventoryDirectory}/go-toolchain/LICENSE`;
const compare = (left, right) => (left < right ? -1 : left > right ? 1 : 0);
const digest = (bytes) => createHash('sha256').update(bytes).digest('hex');
const exactKeys = (value, names) =>
  value !== null &&
  typeof value === 'object' &&
  !Array.isArray(value) &&
  JSON.stringify(Object.keys(value).sort(compare)) === JSON.stringify([...names].sort(compare));
const safeRelative = (value) =>
  typeof value === 'string' &&
  value.length > 0 &&
  value.length <= 512 &&
  !value.includes('\\') &&
  !value.includes(':') &&
  value
    .split('/')
    .every((part) => /^[A-Za-z0-9_.!@+-]+$/u.test(part) && part !== '.' && part !== '..');
const moduleID = (name) =>
  `${name
    .split('/')
    .at(-1)
    .toLowerCase()
    .replace(/[^a-z0-9._-]/gu, '_')
    .slice(0, 32)}-${digest(name).slice(0, 12)}`;
const licenseName = (name) =>
  /^(?:licen[sc]e|copying|notice|copyright|patents)(?:[._-].*)?$/iu.test(name) &&
  !/\.(?:go|js|ts|json|ya?ml|toml|tmpl)$/iu.test(name);

const linkedModules = () => {
  const modules = new Map();
  for (const target of targets) {
    const [goos, goarch] = target.split('/');
    const result = spawnSync(
      'go',
      [
        '-C',
        'apps/api',
        'list',
        '-mod=readonly',
        '-deps',
        '-f',
        '{{with .Module}}{{if not .Main}}{{.Path}}|{{.Version}}|{{.Dir}}|{{if .Replace}}replaced{{end}}{{end}}{{end}}',
        ...commands,
      ],
      {
        cwd: root,
        encoding: 'utf8',
        timeout: 60_000,
        maxBuffer: 8 << 20,
        windowsHide: true,
        env: {
          ...process.env,
          GOOS: goos,
          GOARCH: goarch,
          CGO_ENABLED: '0',
          GOTOOLCHAIN: 'local',
          GOPROXY: 'off',
          GOSUMDB: 'off',
          GOWORK: 'off',
          GOFLAGS: '-mod=readonly',
        },
      },
    );
    if (result.error || result.status !== 0)
      throw new Error(
        `Offline linked-module graph unavailable for ${target}; prepare the pinned tools and bootstrap cache.`,
      );
    for (const line of result.stdout.split(/\r?\n/u).filter(Boolean)) {
      const [name, version, directory, replacement, ...extra] = line.split('|');
      if (
        !name ||
        !/^v[0-9][A-Za-z0-9.+-]*$/u.test(version ?? '') ||
        !directory ||
        replacement ||
        extra.length
      )
        throw new Error('Unsupported linked-module identity or replacement.');
      const existing = modules.get(name);
      if (existing && (existing.version !== version || existing.directory !== directory))
        throw new Error(`Linked-module version mismatch: ${name}`);
      const item = existing ?? { module: name, version, directory, targets: [] };
      if (!item.targets.includes(target)) item.targets.push(target);
      modules.set(name, item);
    }
  }
  if (!modules.size || modules.size > 512)
    throw new Error('Linked-module inventory size is invalid.');
  return [...modules.values()].sort((left, right) => compare(left.module, right.module));
};

const sourceMaterials = async (directory) => {
  const files = [];
  let visited = 0;
  const walk = async (current, depth, licenseDirectory = false) => {
    if (++visited > 20_000 || depth > 32)
      throw new Error('Module license tree exceeds the reviewed bound.');
    const entries = await readdir(current, { withFileTypes: true });
    entries.sort((left, right) => compare(left.name, right.name));
    for (const entry of entries) {
      const path = resolve(current, entry.name);
      if (entry.isSymbolicLink()) throw new Error('Module license tree contains a symbolic link.');
      if (entry.isDirectory()) {
        if (entry.name !== '.git')
          await walk(path, depth + 1, licenseDirectory || /^licen[sc]es$/iu.test(entry.name));
      } else if (entry.isFile() && (licenseDirectory || licenseName(entry.name))) {
        const source = relative(directory, path).split(sep).join('/');
        if (!safeRelative(source) || files.length >= 256)
          throw new Error('Module license file identity exceeds the reviewed bound.');
        const info = await lstat(path);
        if (info.size < 1 || info.size > 2 << 20)
          throw new Error('Module license file size exceeds the reviewed bound.');
        files.push({ source, bytes: await readFile(path) });
      }
    }
  };
  await walk(directory, 0);
  if (!files.length) throw new Error('A linked module has no upstream license material.');
  return files.sort((left, right) => compare(left.source, right.source));
};

const moduleSums = async () =>
  new Map(
    (await readFile(resolve(root, 'apps/api/go.sum'), 'utf8'))
      .trim()
      .split(/\r?\n/u)
      .map((line) => {
        const [name, version, sum] = line.split(' ');
        return [`${name}@${version}`, sum];
      }),
  );

const toolchainMaterial = async () => {
  const pinned = /^toolchain go([0-9]+\.[0-9]+\.[0-9]+)$/mu.exec(
    await readFile(resolve(root, 'apps/api/go.mod'), 'utf8'),
  )?.[1];
  const result = spawnSync('go', ['env', '-json', 'GOVERSION', 'GOROOT'], {
    cwd: root,
    encoding: 'utf8',
    timeout: 5_000,
    maxBuffer: 64 << 10,
    windowsHide: true,
    env: { ...process.env, GOTOOLCHAIN: 'local' },
  });
  if (!pinned || result.error || result.status !== 0)
    throw new Error('Pinned Go distribution unavailable.');
  const toolchain = JSON.parse(result.stdout);
  if (toolchain.GOVERSION !== `go${pinned}` || typeof toolchain.GOROOT !== 'string')
    throw new Error('Pinned Go distribution version mismatch.');
  const path = resolve(toolchain.GOROOT, 'LICENSE');
  const info = await lstat(path);
  if (!info.isFile() || info.isSymbolicLink() || info.size < 1 || info.size > 2 << 20)
    throw new Error('Pinned Go distribution license unavailable.');
  const bytes = await readFile(path);
  return {
    record: {
      version: pinned,
      source: 'Go distribution root/LICENSE',
      path: toolchainPath,
      sha256: digest(bytes),
    },
    bytes,
  };
};

const writeInventory = async (linked) => {
  const sums = await moduleSums();
  const modules = [];
  for (const item of linked) {
    const files = [];
    for (const material of await sourceMaterials(item.directory)) {
      const path = `${inventoryDirectory}/${moduleID(item.module)}/${material.source}`;
      await mkdir(dirname(resolve(root, path)), { recursive: true });
      await writeFile(resolve(root, path), material.bytes);
      files.push({ source: material.source, path, sha256: digest(material.bytes) });
    }
    const checksum = sums.get(`${item.module}@${item.version}`);
    if (!/^h1:[A-Za-z0-9+/]{43}=$/u.test(checksum ?? ''))
      throw new Error(`Pinned module checksum missing: ${item.module}`);
    modules.push({
      module: item.module,
      version: item.version,
      checksum,
      targets: item.targets,
      files,
    });
  }
  const toolchain = await toolchainMaterial();
  await mkdir(dirname(resolve(root, toolchainPath)), { recursive: true });
  await writeFile(resolve(root, toolchainPath), toolchain.bytes);
  const manifest = {
    schema_version: 1,
    scope,
    commands,
    targets,
    cgo_enabled: false,
    toolchain: toolchain.record,
    modules,
  };
  await mkdir(resolve(root, inventoryDirectory), { recursive: true });
  await writeFile(resolve(root, manifestPath), `${JSON.stringify(manifest, null, 2)}\n`);
};

const verifyInventory = async (linked) => {
  const manifest = JSON.parse(await readFile(resolve(root, manifestPath), 'utf8'));
  if (
    !exactKeys(manifest, [
      'schema_version',
      'scope',
      'commands',
      'targets',
      'cgo_enabled',
      'toolchain',
      'modules',
    ]) ||
    manifest.schema_version !== 1 ||
    manifest.scope !== scope ||
    manifest.cgo_enabled !== false ||
    JSON.stringify(manifest.commands) !== JSON.stringify(commands) ||
    JSON.stringify(manifest.targets) !== JSON.stringify(targets) ||
    !Array.isArray(manifest.modules) ||
    manifest.modules.length !== linked.length
  )
    throw new Error('Go notice inventory scope or module graph drift; regenerate with --write.');
  const toolchain = await toolchainMaterial();
  if (
    !exactKeys(manifest.toolchain, ['version', 'source', 'path', 'sha256']) ||
    JSON.stringify(manifest.toolchain) !== JSON.stringify(toolchain.record) ||
    digest(await readFile(resolve(root, toolchainPath))) !== toolchain.record.sha256
  )
    throw new Error('Pinned Go distribution license drift.');
  const sums = await moduleSums();
  let fileCount = 1;
  for (let index = 0; index < linked.length; index++) {
    const actual = manifest.modules[index];
    const expected = linked[index];
    if (
      !exactKeys(actual, ['module', 'version', 'checksum', 'targets', 'files']) ||
      actual.module !== expected.module ||
      actual.version !== expected.version ||
      actual.checksum !== sums.get(`${expected.module}@${expected.version}`) ||
      JSON.stringify(actual.targets) !== JSON.stringify(expected.targets)
    )
      throw new Error(`Go notice module/version/checksum drift: ${expected.module}`);
    if (!Array.isArray(actual.files) || actual.files.length < 1 || actual.files.length > 256)
      throw new Error(`Go notice files missing: ${expected.module}`);
    const upstream = await sourceMaterials(expected.directory);
    if (actual.files.length !== upstream.length)
      throw new Error(`Go notice upstream material set drift: ${expected.module}`);
    let previous = '';
    for (let fileIndex = 0; fileIndex < actual.files.length; fileIndex++) {
      const file = actual.files[fileIndex];
      const expectedPath = `${inventoryDirectory}/${moduleID(expected.module)}/${file.source}`;
      if (
        !exactKeys(file, ['source', 'path', 'sha256']) ||
        !safeRelative(file.source) ||
        !safeRelative(file.path) ||
        file.path !== expectedPath ||
        !/^[a-f0-9]{64}$/u.test(file.sha256 ?? '') ||
        compare(previous, file.source) >= 0
      )
        throw new Error(`Unsafe or duplicate Go notice file: ${expected.module}`);
      if (
        file.source !== upstream[fileIndex].source ||
        file.sha256 !== digest(upstream[fileIndex].bytes)
      )
        throw new Error(`Go notice upstream bytes drift: ${expected.module}`);
      const info = await lstat(resolve(root, file.path));
      if (
        !info.isFile() ||
        info.isSymbolicLink() ||
        info.size < 1 ||
        info.size > 2 << 20 ||
        digest(await readFile(resolve(root, file.path))) !== file.sha256
      )
        throw new Error(`Go notice copy hash mismatch: ${expected.module}/${file.source}`);
      previous = file.source;
      fileCount++;
    }
  }
  return fileCount;
};

try {
  if (
    process.argv.slice(2).some((argument) => argument !== '--write') ||
    process.argv.slice(2).length > 1
  )
    throw new Error('Usage: node scripts/check-go-notices.mjs [--write]');
  const linked = linkedModules();
  if (process.argv.includes('--write')) await writeInventory(linked);
  const files = await verifyInventory(linked);
  process.stdout.write(
    `Go linked-module notices pass: ${linked.length} modules, ${files} exact upstream files, ${targets.join(' + ')}.\n`,
  );
} catch (error) {
  const safe =
    error instanceof Error && !error.code && !(error instanceof SyntaxError)
      ? error.message
      : 'Go notice inventory files unavailable or invalid; bootstrap or regenerate the inventory.';
  process.stderr.write(`${safe}\n`);
  process.exitCode = 1;
}
