import { createHash } from 'node:crypto';
import { createRequire } from 'node:module';
import { lstat, mkdir, readFile, readdir, realpath, writeFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

const root = path.resolve(import.meta.dirname, '..');
const inventory = 'apps/web/public/licenses/packages';
const manifestPath = `${inventory}/manifest.json`;
const roots = ['apps/web', 'packages/ui', 'packages/api-client'];
const scope =
  'Installed external packages reachable through declared production dependencies, installed optional dependencies and installed peers of the web/ui/api-client workspaces; exact upstream legal material, not a legal interpretation or a bundle tree-shaking inventory. Excludes development-only roots, Go modules and container system packages.';
const compare = (a, b) => (a < b ? -1 : a > b ? 1 : 0);
const sha256 = (bytes) => createHash('sha256').update(bytes).digest('hex');
const safeRelative = (value) =>
  value.length > 0 &&
  value.length <= 512 &&
  !value.includes('\\') &&
  !value.includes(':') &&
  value
    .split('/')
    .every((part) => /^[A-Za-z0-9_.!@+-]+$/u.test(part) && part !== '.' && part !== '..');
const withinRoot = (directory) => {
  const relative = path.relative(root, directory);
  return relative !== '..' && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
};
const idFor = (name, version) =>
  `${name.replace(/[^a-z0-9._-]/gu, '_').slice(0, 48)}-${sha256(`${name}@${version}`).slice(0, 12)}`;
const licenseName = (name) =>
  /^(?:licen[sc]e|copying|notice|copyright|patents|ofl)(?:[._-].*)?$/iu.test(name) &&
  !/\.(?:js|ts|json|map|css|go|wasm)$/iu.test(name);
// Explicitly reviewed additive notice for a package that publishes no full license.
// No other package/version or installed manifest can use this supplement.
const supplements = {
  'react-remove-scroll-bar@2.3.8': {
    package_json_sha256: 'e372f857ce05f266137c65293436b5380fec42ac311e6acb9378ed78b98d75d0',
    path: 'apps/web/licenses/upstream/react-remove-scroll-bar-2.3.8-LICENSE.txt',
    source_url:
      'https://raw.githubusercontent.com/theKashey/react-remove-scroll-bar/7301c160fda44cb8cf2b9fdfde61efad35736196/LICENSE',
    source_commit: '7301c160fda44cb8cf2b9fdfde61efad35736196',
    sha256: 'a79aae0c0f21990d9d963bb3c5a79cdcea9a46f8523ba55c58d7fe776b6ebc84',
    reason:
      'Published 2.3.8 package.json and README declare MIT and identify Anton Korzunov but omit full legal text. The same upstream author added this MIT LICENSE on 2025-05-21, after the 2024-12-15 package publication. This is reviewed supplemental attribution, not a claim that this file shipped in the 2024 package.',
  },
};

const readManifest = async (directory) => {
  const filename = path.join(directory, 'package.json');
  const info = await lstat(filename);
  if (!info.isFile() || info.isSymbolicLink() || info.size < 1 || info.size > 256 << 10)
    throw new Error('Package manifest is invalid.');
  const bytes = await readFile(filename);
  const value = JSON.parse(bytes);
  if (
    typeof value.name !== 'string' ||
    typeof value.version !== 'string' ||
    value.name.length > 256 ||
    value.version.length > 128
  )
    throw new Error('Package identity is invalid.');
  return { value, hash: sha256(bytes) };
};

const resolvePackage = async (name, parent, optional) => {
  if (!/^(@[a-z0-9_.-]+\/)?[a-z0-9_.-]+$/iu.test(name)) throw new Error('Unsafe dependency name.');
  // Inspect installed package manifests directly; public exports may hide package.json.
  const require = createRequire(path.join(parent, 'package.json'));
  for (const lookup of require.resolve.paths(name) ?? []) {
    const directory = path.join(lookup, name);
    const exists = await lstat(path.join(directory, 'package.json')).catch((error) => {
      if (error.code === 'ENOENT') return null;
      throw error;
    });
    if (exists === null) continue;
    const resolved = await realpath(directory);
    if (!withinRoot(resolved))
      throw new Error(`Installed dependency leaves the workspace: ${name}`);
    const manifest = await readManifest(resolved);
    if (manifest.value.name !== name)
      throw new Error(`Installed package identity mismatch: ${name}`);
    return { directory: resolved, ...manifest };
  }
  if (optional) return null;
  throw new Error(`Production dependency is not installed: ${name}`);
};

const sourceMaterials = async (directory) => {
  const files = [];
  let visited = 0;
  const walk = async (current, depth, legalDirectory = false) => {
    if (++visited > 20_000 || depth > 20)
      throw new Error('Package legal tree exceeds the reviewed bound.');
    const entries = await readdir(current, { withFileTypes: true });
    entries.sort((a, b) => compare(a.name, b.name));
    for (const entry of entries) {
      if (entry.name === 'node_modules' || entry.name === '.git') continue;
      const filename = path.join(current, entry.name);
      if (entry.isSymbolicLink()) throw new Error('Package legal tree contains a symbolic link.');
      if (entry.isDirectory()) {
        await walk(filename, depth + 1, legalDirectory || /^licen[sc]es$/iu.test(entry.name));
      } else if (entry.isFile() && (legalDirectory || licenseName(entry.name))) {
        const source = path.relative(directory, filename).split(path.sep).join('/');
        const info = await lstat(filename);
        if (!safeRelative(source) || files.length >= 128 || info.size < 1 || info.size > 2 << 20)
          throw new Error('Package legal file exceeds the reviewed bound.');
        files.push({ source, bytes: await readFile(filename) });
      }
    }
  };
  await walk(directory, 0);
  return files.sort((a, b) => compare(a.source, b.source));
};

const expectedInventory = async () => {
  const queue = [];
  for (const source of roots) {
    const directory = await realpath(path.join(root, source));
    queue.push({ directory, ...(await readManifest(directory)) });
  }
  const visited = new Set();
  const workspaceDirectories = new Set(queue.map((item) => item.directory));
  const external = new Map();
  for (let index = 0; index < queue.length; index++) {
    const item = queue[index];
    if (visited.has(item.directory)) continue;
    visited.add(item.directory);
    if (visited.size > 512) throw new Error('Production package graph exceeds the reviewed bound.');
    const value = item.value;
    if (!workspaceDirectories.has(item.directory)) {
      const key = `${value.name}@${value.version}`;
      const existing = external.get(key);
      if (existing && existing.hash !== item.hash)
        throw new Error('Duplicate package version has conflicting installed metadata.');
      external.set(key, item);
    }
    const dependencies = new Map();
    for (const name of Object.keys(value.dependencies ?? {})) dependencies.set(name, false);
    for (const name of Object.keys(value.optionalDependencies ?? {}))
      if (!dependencies.has(name)) dependencies.set(name, true);
    for (const name of Object.keys(value.peerDependencies ?? {}))
      if (!dependencies.has(name)) dependencies.set(name, true);
    for (const [name, optional] of [...dependencies].sort(([a], [b]) => compare(a, b))) {
      const dependency = await resolvePackage(name, item.directory, optional);
      if (dependency !== null) queue.push(dependency);
    }
  }
  const packages = [];
  const copies = new Map();
  const missing = [];
  for (const [identity, item] of [...external].sort(([a], [b]) => compare(a, b))) {
    const { name, version, license } = item.value;
    const files = [];
    let materials = await sourceMaterials(item.directory);
    let supplement = null;
    if (materials.length === 0 && Object.hasOwn(supplements, identity)) {
      supplement = supplements[identity];
      if (supplement.package_json_sha256 !== item.hash || license !== 'MIT')
        throw new Error('Reviewed supplemental notice package identity drift.');
      const filename = path.join(root, supplement.path);
      const info = await lstat(filename);
      const bytes = await readFile(filename);
      if (
        !info.isFile() ||
        info.isSymbolicLink() ||
        info.size < 1 ||
        info.size > 2 << 20 ||
        sha256(bytes) !== supplement.sha256
      )
        throw new Error('Reviewed supplemental legal text hash mismatch.');
      materials = [{ source: 'LICENSE', bytes }];
    }
    if (materials.length === 0) missing.push(identity);
    for (const material of materials) {
      const target = `${inventory}/${idFor(name, version)}/${material.source}`;
      copies.set(target, material.bytes);
      files.push({ source: material.source, path: target, sha256: sha256(material.bytes) });
    }
    packages.push({
      identity,
      name,
      version,
      declared_license: typeof license === 'string' ? license : null,
      package_json_sha256: item.hash,
      legal_origin: supplement === null ? 'installed-package' : 'reviewed-upstream-supplement',
      supplement,
      files,
    });
  }
  if (missing.length > 0)
    throw new Error(
      `Packages have no upstream legal material; explicit review required: ${missing.join(', ')}`,
    );
  return {
    manifest: {
      schema_version: 1,
      scope,
      roots,
      lockfile_sha256: sha256(await readFile(path.join(root, 'pnpm-lock.yaml'))),
      packages,
    },
    copies,
  };
};

const verifyOutputFileSet = async (expected) => {
  const allowed = new Set([manifestPath, ...expected.copies.keys()]);
  let visited = 0;
  const walk = async (directory) => {
    if (++visited > 4096) throw new Error('Web legal output tree exceeds the reviewed bound.');
    for (const entry of await readdir(directory, { withFileTypes: true })) {
      const filename = path.join(directory, entry.name);
      if (entry.isSymbolicLink()) throw new Error('Web legal output contains a symbolic link.');
      if (entry.isDirectory()) await walk(filename);
      else if (
        !entry.isFile() ||
        !allowed.has(path.relative(root, filename).split(path.sep).join('/'))
      )
        throw new Error(
          'Web legal output contains an unreviewed or stale file; review it before removing.',
        );
    }
  };
  await walk(path.join(root, inventory));
};

try {
  const expected = await expectedInventory();
  const bytes = `${JSON.stringify(expected.manifest, null, 2)}\n`;
  if (process.argv.includes('--write')) {
    for (const [target, content] of expected.copies) {
      await mkdir(path.dirname(path.join(root, target)), { recursive: true });
      await writeFile(path.join(root, target), content);
    }
    await mkdir(path.join(root, inventory), { recursive: true });
    await writeFile(path.join(root, manifestPath), bytes);
  } else {
    await verifyOutputFileSet(expected);
    if ((await readFile(path.join(root, manifestPath), 'utf8')) !== bytes)
      throw new Error('Web production notices graph/metadata drift; regenerate with --write.');
    for (const [target, content] of expected.copies) {
      const filename = path.join(root, target);
      const info = await lstat(filename);
      if (
        !info.isFile() ||
        info.isSymbolicLink() ||
        sha256(await readFile(filename)) !== sha256(content)
      )
        throw new Error(`Web legal copy differs from exact installed upstream bytes: ${target}`);
    }
  }
  process.stdout.write(
    `Web production notices pass: ${expected.manifest.packages.length} exact packages, ${expected.copies.size} upstream legal files.\n`,
  );
} catch (error) {
  process.stderr.write(
    `${error instanceof Error ? error.message : 'Web production notices check failed.'}\n`,
  );
  process.exitCode = 1;
}
