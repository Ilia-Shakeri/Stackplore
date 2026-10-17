import { lstat, readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';

export function checkReleaseScan(report, sbom, { image = false, extension = '' } = {}) {
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
  for (const name of image ? ['server'] : ['server', 'storage', 'operator']) {
    if (!binaries.has(`stackplore-${name}${extension}`))
      throw new Error('Release scan did not analyze every shipped Go executable.');
  }
  if (
    image &&
    !report.Results.some((result) => result.Class === 'os-pkgs' && typeof result.Type === 'string')
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
          /^pkg:golang\/(?!stdlib@).+@/u.test(reference.referenceLocator),
      ),
    )
  )
    throw new Error('Release SBOM contains no Go module package URLs.');
}

async function readReport(path, maximum) {
  const info = await lstat(path);
  if (!info.isFile() || info.isSymbolicLink() || info.size > maximum)
    throw new Error('Release scan document is not a bounded regular file.');
  return JSON.parse(await readFile(path, 'utf8'));
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  const [kind, reportPath, sbomPath, extension = ''] = process.argv.slice(2);
  if (
    !['native', 'image'].includes(kind) ||
    !reportPath ||
    !sbomPath ||
    !['', '.exe'].includes(extension)
  )
    throw new Error('Expected release scan kind, report, SPDX document and optional .exe suffix.');
  checkReleaseScan(
    await readReport(reportPath, 32 * 1024 * 1024),
    await readReport(sbomPath, 16 * 1024 * 1024),
    {
      image: kind === 'image',
      extension,
    },
  );
  process.stdout.write('Release scan analyzed the shipped Go tools and inventoried Go modules.\n');
}
