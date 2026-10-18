import { lstat, readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';
import { validatePostgresImagePolicy } from './runtime-image-policy.mjs';

export function checkReleaseScan(
  report,
  sbom,
  { image = false, database = false, extension = '' } = {},
) {
  if (!Array.isArray(report?.Results)) throw new Error('Release scan has no analyzed results.');
  const binaries = new Set(
    report.Results.filter(
      (result) => result.Class === 'lang-pkgs' && result.Type === 'gobinary',
    ).map((result) =>
      String(result.Target)
        .replaceAll('\\', '/')
        .replace(/ \(gobinary\)$/u, '')
        .split('/')
        .at(-1),
    ),
  );
  for (const name of database ? [] : image ? ['server'] : ['server', 'storage', 'operator']) {
    if (!binaries.has(`stackplore-${name}${extension}`))
      throw new Error('Release scan did not analyze every shipped Go executable.');
  }
  if (
    (image || database) &&
    !report.Results.some(
      (result) =>
        result.Class === 'os-pkgs' &&
        (database ? result.Type === 'alpine' : typeof result.Type === 'string'),
    )
  )
    throw new Error('Container release scan has no operating-system package result.');
  if (
    report.Results.some((result) =>
      result.Vulnerabilities?.some((finding) => ['HIGH', 'CRITICAL'].includes(finding.Severity)),
    )
  )
    throw new Error('Release scan still contains a blocking vulnerability.');
  if (
    !Array.isArray(sbom?.packages) ||
    !sbom.packages.some((entry) =>
      entry.externalRefs?.some(
        (reference) =>
          reference.referenceType === 'purl' &&
          (database ? /^pkg:apk\/alpine\/[^@]+@/u : /^pkg:golang\/(?!stdlib@).+@/u).test(
            reference.referenceLocator,
          ),
      ),
    )
  )
    throw new Error(
      database
        ? 'Database SBOM contains no Alpine package URLs.'
        : 'Release SBOM contains no Go module package URLs.',
    );
}

const digestPattern = /^sha256:[a-f0-9]{64}$/u;

export function checkDatabasePin(pin) {
  return validatePostgresImagePolicy(pin);
}

export function createDatabaseImageMetadata(pin, inspection, index, probe) {
  checkDatabasePin(pin);
  const image = Array.isArray(inspection) && inspection.length === 1 ? inspection[0] : undefined;
  const manifests = index?.manifests?.filter(
    (entry) =>
      entry.platform?.os === 'linux' &&
      entry.platform?.architecture === 'amd64' &&
      !entry.platform?.variant,
  );
  const upstreamDigest = pin.image.split('@')[1];
  if (
    image?.Os !== 'linux' ||
    image?.Architecture !== 'amd64' ||
    !digestPattern.test(image?.Id ?? '') ||
    !image.RepoDigests?.some((reference) =>
      [`postgres@${upstreamDigest}`, `docker.io/library/postgres@${upstreamDigest}`].includes(
        reference,
      ),
    ) ||
    manifests?.length !== 1 ||
    !digestPattern.test(manifests[0]?.digest ?? '')
  )
    throw new Error('Database inspection does not match the pinned upstream image and platform.');
  const metadata = {
    schema_version: 1,
    image: pin.image,
    upstream_index_digest: upstreamDigest,
    platform: pin.platform,
    platform_manifest_digest: manifests[0].digest,
    config_digest: image.Id,
    runtime_probe: {
      entrypoint: 'postgres',
      arguments: ['--version'],
      user: '70:70',
      network: 'none',
      read_only: true,
      cap_drop: ['ALL'],
      no_new_privileges: true,
      exit_code: 0,
      output: probe.trim(),
      version: pin.version,
    },
  };
  checkDatabaseImageMetadata(metadata, pin);
  return metadata;
}

export function checkDatabaseImageMetadata(metadata, pin) {
  checkDatabasePin(pin);
  const probe = metadata?.runtime_probe;
  if (
    metadata?.schema_version !== 1 ||
    metadata.image !== pin.image ||
    metadata.upstream_index_digest !== pin.image.split('@')[1] ||
    metadata.platform !== pin.platform ||
    !digestPattern.test(metadata.platform_manifest_digest ?? '') ||
    !digestPattern.test(metadata.config_digest ?? '') ||
    probe?.entrypoint !== 'postgres' ||
    JSON.stringify(probe.arguments) !== '["--version"]' ||
    probe.user !== '70:70' ||
    probe.network !== 'none' ||
    probe.read_only !== true ||
    JSON.stringify(probe.cap_drop) !== '["ALL"]' ||
    probe.no_new_privileges !== true ||
    probe.exit_code !== 0 ||
    probe.version !== pin.version ||
    probe.output !== `postgres (PostgreSQL) ${pin.version}`
  )
    throw new Error(
      'Database runtime metadata does not prove the approved engine version and image.',
    );
}

export function checkDatabaseScanIdentity(report, metadata) {
  if (
    report?.ArtifactName !== metadata.image ||
    report?.Metadata?.ImageID !== metadata.config_digest ||
    report.Metadata.OS?.Family !== 'alpine'
  )
    throw new Error('Database scan does not identify the probed image configuration.');
}

export function checkDatabaseSbomIdentity(sbom, metadata, report) {
  const packages = Array.isArray(sbom?.packages) ? sbom.packages : [];
  const relationships = Array.isArray(sbom?.relationships) ? sbom.relationships : [];
  const roots = packages.filter((entry) => entry.primaryPackagePurpose === 'CONTAINER');
  const root = roots[0];
  const comments = Array.isArray(root?.annotations)
    ? root.annotations
        .map((entry) => entry.comment)
        .filter((comment) => typeof comment === 'string')
    : [];
  const describes = relationships.filter(
    (entry) => entry.spdxElementId === sbom?.SPDXID && entry.relationshipType === 'DESCRIBES',
  );
  if (
    sbom?.spdxVersion !== 'SPDX-2.3' ||
    sbom.SPDXID !== 'SPDXRef-DOCUMENT' ||
    sbom.name !== metadata.image ||
    roots.length !== 1 ||
    root.name !== metadata.image ||
    typeof root.SPDXID !== 'string' ||
    root.SPDXID === sbom.SPDXID ||
    comments.filter((comment) => comment.startsWith('ImageID: ')).length !== 1 ||
    !comments.includes(`ImageID: ${metadata.config_digest}`) ||
    comments.filter((comment) => comment.startsWith('Reference: ')).length !== 1 ||
    !comments.includes(`Reference: ${metadata.image}`) ||
    describes.length !== 1 ||
    describes[0].relatedSpdxElement !== root.SPDXID
  )
    throw new Error('Database SPDX does not identify the probed image configuration.');

  const packageById = new Map(packages.map((entry) => [entry.SPDXID, entry]));
  const systems = packages.filter((entry) => entry.primaryPackagePurpose === 'OPERATING-SYSTEM');
  const system = systems[0];
  const contains = (parent, child) =>
    relationships.some(
      (entry) =>
        entry.spdxElementId === parent &&
        entry.relatedSpdxElement === child &&
        entry.relationshipType === 'CONTAINS',
    );
  if (
    packageById.size !== packages.length ||
    packages.some((entry) => typeof entry.SPDXID !== 'string' || !entry.SPDXID) ||
    systems.length !== 1 ||
    system.name !== 'alpine' ||
    typeof report?.Metadata?.OS?.Name !== 'string' ||
    !report.Metadata.OS.Name ||
    system.versionInfo !== report.Metadata.OS.Name ||
    !contains(root.SPDXID, system.SPDXID)
  )
    throw new Error('Database SPDX operating system is not linked to the scanned image.');

  const alpinePurls = (entry) =>
    (Array.isArray(entry?.externalRefs) ? entry.externalRefs : [])
      .filter(
        (reference) =>
          reference.referenceType === 'purl' &&
          typeof reference.referenceLocator === 'string' &&
          /^pkg:apk\/alpine\/[^@]+@/u.test(reference.referenceLocator),
      )
      .map((reference) => reference.referenceLocator);
  const dependencies = new Map();
  for (const entry of relationships) {
    if (!['CONTAINS', 'DEPENDS_ON'].includes(entry.relationshipType)) continue;
    const children = dependencies.get(entry.spdxElementId) ?? [];
    children.push(entry.relatedSpdxElement);
    dependencies.set(entry.spdxElementId, children);
  }
  const visited = new Set([system.SPDXID]);
  const pending = [system.SPDXID];
  for (let index = 0; index < pending.length; index++) {
    for (const id of dependencies.get(pending[index]) ?? []) {
      if (!visited.has(id)) {
        visited.add(id);
        pending.push(id);
      }
    }
  }
  const containedIds = pending.slice(1);
  const containedPurls = containedIds.flatMap((id) => alpinePurls(packageById.get(id)));
  const scannedPackages = report.Results.filter(
    (entry) => entry.Class === 'os-pkgs' && entry.Type === 'alpine',
  ).flatMap((entry) => (Array.isArray(entry.Packages) ? entry.Packages : []));
  const scannedPurls = scannedPackages.map((entry) => entry.Identifier?.PURL);
  const sameSet = (left, right) => {
    const values = new Set(left);
    return values.size === new Set(right).size && right.every((value) => values.has(value));
  };
  if (
    containedIds.length === 0 ||
    containedIds.some((id) => alpinePurls(packageById.get(id)).length !== 1) ||
    scannedPackages.length === 0 ||
    scannedPurls.some(
      (purl) => typeof purl !== 'string' || !/^pkg:apk\/alpine\/[^@]+@/u.test(purl),
    ) ||
    !sameSet(containedPurls, scannedPurls) ||
    !sameSet(containedPurls, packages.flatMap(alpinePurls))
  )
    throw new Error(
      'Database SPDX package inventory does not match the analyzed operating-system packages.',
    );
}

async function readText(path, maximum) {
  const info = await lstat(path);
  if (!info.isFile() || info.isSymbolicLink() || info.size > maximum)
    throw new Error('Release scan document is not a bounded regular file.');
  return readFile(path, 'utf8');
}

async function readReport(path, maximum) {
  return JSON.parse(await readText(path, maximum));
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  if (process.argv[2] === 'database-reference') {
    const pin = checkDatabasePin(await readReport(process.argv[3], 4096));
    process.stdout.write(`${pin.image}\n`);
  } else if (process.argv[2] === 'database-metadata') {
    const [pinPath, inspectionPath, indexPath, probePath, outputPath] = process.argv.slice(3);
    if (!pinPath || !inspectionPath || !indexPath || !probePath || !outputPath)
      throw new Error(
        'Expected database pin, inspection, upstream index, version probe and output paths.',
      );
    const metadata = createDatabaseImageMetadata(
      await readReport(pinPath, 4096),
      await readReport(inspectionPath, 1024 * 1024),
      await readReport(indexPath, 1024 * 1024),
      await readText(probePath, 4096),
    );
    await writeFile(outputPath, `${JSON.stringify(metadata, null, 2)}\n`);
  } else {
    const [kind, reportPath, sbomPath, extra = ''] = process.argv.slice(2);
    if (
      !['native', 'image', 'database'].includes(kind) ||
      !reportPath ||
      !sbomPath ||
      (kind === 'database' ? !extra : !['', '.exe'].includes(extra))
    )
      throw new Error(
        'Expected release scan kind, report, SPDX document and optional .exe suffix.',
      );
    const report = await readReport(reportPath, 32 * 1024 * 1024);
    const sbom = await readReport(sbomPath, 16 * 1024 * 1024);
    checkReleaseScan(report, sbom, {
      image: kind === 'image',
      database: kind === 'database',
      extension: kind === 'database' ? '' : extra,
    });
    if (kind === 'database') {
      const metadata = await readReport(extra, 16 * 1024);
      checkDatabaseImageMetadata(metadata, await readReport('deploy/postgres-image.json', 4096));
      checkDatabaseScanIdentity(report, metadata);
      checkDatabaseSbomIdentity(sbom, metadata, report);
    }
    process.stdout.write(
      kind === 'database'
        ? 'Database scan analyzed Alpine packages and inventoried package URLs.\n'
        : 'Release scan analyzed the shipped Go tools and inventoried Go modules.\n',
    );
  }
}
