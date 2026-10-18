import { lstat, readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';
import { GOSU_BUILD_IMAGE, validatePostgresImagePolicy } from './runtime-image-policy.mjs';

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

const databaseBuildInputs = (pin) => ({
  dockerfile: 'deploy/Dockerfile.postgres',
  gosu_version: pin.gosu_version,
  gosu_source_commit: pin.gosu_source_commit,
  gosu_source_sha256: pin.gosu_source_sha256,
  gosu_build_go: pin.gosu_build_go,
  gosu_build_image: GOSU_BUILD_IMAGE,
});

const isolatedProbe = (entrypoint, output, version) => ({
  entrypoint,
  arguments: ['--version'],
  user: '70:70',
  network: 'none',
  read_only: true,
  cap_drop: ['ALL'],
  no_new_privileges: true,
  exit_code: 0,
  output: output.trim(),
  version,
});

export function createDatabaseImageMetadata(
  pin,
  inspection,
  index,
  runtimeInspection,
  probe,
  gosuProbe,
) {
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
  const runtime =
    Array.isArray(runtimeInspection) && runtimeInspection.length === 1
      ? runtimeInspection[0]
      : undefined;
  if (
    runtime?.Os !== 'linux' ||
    runtime?.Architecture !== 'amd64' ||
    !digestPattern.test(runtime?.Id ?? '') ||
    runtime.Id === image.Id ||
    !runtime.RepoTags?.includes(pin.runtime_image)
  )
    throw new Error('Database runtime inspection does not identify the built derivative image.');
  const metadata = {
    schema_version: 1,
    image: pin.runtime_image,
    upstream_image: pin.image,
    upstream_index_digest: upstreamDigest,
    upstream_platform_manifest_digest: manifests[0].digest,
    upstream_config_digest: image.Id,
    platform: pin.platform,
    config_digest: runtime.Id,
    build_inputs: databaseBuildInputs(pin),
    runtime_probe: isolatedProbe('postgres', probe, pin.version),
    gosu_probe: {
      ...isolatedProbe('gosu', gosuProbe, pin.gosu_version),
      go_version: pin.gosu_build_go,
    },
  };
  checkDatabaseImageMetadata(metadata, pin);
  return metadata;
}

export function checkDatabaseImageMetadata(metadata, pin) {
  checkDatabasePin(pin);
  const probe = metadata?.runtime_probe;
  const gosu = metadata?.gosu_probe;
  const expectedBuildInputs = databaseBuildInputs(pin);
  const isIsolated = (value, entrypoint) =>
    value?.entrypoint === entrypoint &&
    JSON.stringify(value.arguments) === '["--version"]' &&
    value.user === '70:70' &&
    value.network === 'none' &&
    value.read_only === true &&
    JSON.stringify(value.cap_drop) === '["ALL"]' &&
    value.no_new_privileges === true &&
    value.exit_code === 0;
  if (
    metadata?.schema_version !== 1 ||
    metadata.image !== pin.runtime_image ||
    metadata.upstream_image !== pin.image ||
    metadata.upstream_index_digest !== pin.image.split('@')[1] ||
    metadata.platform !== pin.platform ||
    !digestPattern.test(metadata.upstream_platform_manifest_digest ?? '') ||
    !digestPattern.test(metadata.upstream_config_digest ?? '') ||
    !digestPattern.test(metadata.config_digest ?? '') ||
    metadata.config_digest === metadata.upstream_config_digest ||
    !metadata.build_inputs ||
    Object.keys(metadata.build_inputs).length !== Object.keys(expectedBuildInputs).length ||
    Object.entries(expectedBuildInputs).some(
      ([key, value]) => metadata.build_inputs[key] !== value,
    ) ||
    !isIsolated(probe, 'postgres') ||
    probe.version !== pin.version ||
    probe.output !== `postgres (PostgreSQL) ${pin.version}` ||
    !isIsolated(gosu, 'gosu') ||
    gosu.version !== pin.gosu_version ||
    gosu.go_version !== pin.gosu_build_go ||
    gosu.output !== `${pin.gosu_version} (go${pin.gosu_build_go} on linux/amd64; gc)`
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
  const gosu = report.Results?.filter(
    (entry) =>
      entry.Class === 'lang-pkgs' &&
      entry.Type === 'gobinary' &&
      String(entry.Target)
        .replaceAll('\\', '/')
        .replace(/ \(gobinary\)$/u, '')
        .split('/')
        .at(-1) === 'gosu',
  );
  const standardLibraries = gosu?.[0]?.Packages?.filter((entry) => entry.Name === 'stdlib');
  if (
    gosu?.length !== 1 ||
    standardLibraries?.length !== 1 ||
    standardLibraries[0].Version !== `v${metadata.build_inputs.gosu_build_go}` ||
    standardLibraries[0].Identifier?.PURL !==
      `pkg:golang/stdlib@v${metadata.build_inputs.gosu_build_go}`
  )
    throw new Error('Database scan does not inventory gosu with the approved Go standard library.');
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
  const normalizedReference = metadata.image.includes('@')
    ? metadata.image.replace(/:[^:/@]+@/u, '@')
    : metadata.image;
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
    ![metadata.image, normalizedReference].some((reference) =>
      comments.includes(`Reference: ${reference}`),
    ) ||
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
  const reachableIds = (start) => {
    const visited = new Set([start]);
    const pending = [start];
    for (let index = 0; index < pending.length; index++) {
      for (const id of dependencies.get(pending[index]) ?? []) {
        if (!visited.has(id)) {
          visited.add(id);
          pending.push(id);
        }
      }
    }
    return pending.slice(1);
  };
  const containedIds = reachableIds(system.SPDXID);
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

  const gosuReport = report.Results.find(
    (entry) =>
      entry.Class === 'lang-pkgs' &&
      entry.Type === 'gobinary' &&
      String(entry.Target)
        .replaceAll('\\', '/')
        .replace(/ \(gobinary\)$/u, '')
        .split('/')
        .at(-1) === 'gosu',
  );
  const applications = packages.filter(
    (entry) => entry.primaryPackagePurpose === 'APPLICATION' && entry.name === gosuReport?.Target,
  );
  const application = applications[0];
  const goPurls = (entry) =>
    (Array.isArray(entry?.externalRefs) ? entry.externalRefs : [])
      .filter(
        (reference) =>
          reference.referenceType === 'purl' &&
          typeof reference.referenceLocator === 'string' &&
          /^pkg:golang\/.+@/u.test(reference.referenceLocator),
      )
      .map((reference) => reference.referenceLocator);
  const gosuPurls = gosuReport?.Packages?.map((entry) => entry.Identifier?.PURL);
  if (
    applications.length !== 1 ||
    !contains(root.SPDXID, application.SPDXID) ||
    !application.annotations?.some((entry) => entry.comment === 'Type: gobinary') ||
    !Array.isArray(gosuPurls) ||
    gosuPurls.length === 0 ||
    gosuPurls.some((purl) => typeof purl !== 'string' || !/^pkg:golang\/.+@/u.test(purl)) ||
    !sameSet(
      reachableIds(application.SPDXID).flatMap((id) => goPurls(packageById.get(id))),
      gosuPurls,
    ) ||
    !sameSet(packages.flatMap(goPurls), gosuPurls)
  )
    throw new Error('Database SPDX gosu inventory does not match the analyzed Go packages.');
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
    const [
      pinPath,
      inspectionPath,
      indexPath,
      runtimeInspectionPath,
      probePath,
      gosuProbePath,
      outputPath,
    ] = process.argv.slice(3);
    if (
      !pinPath ||
      !inspectionPath ||
      !indexPath ||
      !runtimeInspectionPath ||
      !probePath ||
      !gosuProbePath ||
      !outputPath
    )
      throw new Error(
        'Expected database pin, inspection, upstream index, version probe and output paths.',
      );
    const metadata = createDatabaseImageMetadata(
      await readReport(pinPath, 4096),
      await readReport(inspectionPath, 1024 * 1024),
      await readReport(indexPath, 1024 * 1024),
      await readReport(runtimeInspectionPath, 1024 * 1024),
      await readText(probePath, 4096),
      await readText(gosuProbePath, 4096),
    );
    await writeFile(outputPath, `${JSON.stringify(metadata, null, 2)}\n`);
  } else if (process.argv[2] === 'database-runtime-reference') {
    const pin = checkDatabasePin(await readReport(process.argv[3], 4096));
    process.stdout.write(`${pin.runtime_image}\n`);
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
