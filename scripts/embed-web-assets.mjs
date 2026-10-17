import { cp, lstat, mkdir, readdir, realpath, rm } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const source = path.join(root, 'apps', 'web', 'dist');
const destination = path.join(root, 'apps', 'api', 'internal', 'webui', 'static');
const expectedDestination = path.resolve(root, 'apps', 'api', 'internal', 'webui', 'static');

const sourceReal = await realpath(source);
const rootReal = await realpath(root);
const expectedSource = path.resolve(rootReal, 'apps', 'web', 'dist');
const destinationParent = path.dirname(destination);
const destinationParentReal = await realpath(destinationParent);
const sourceEntries = await readdir(sourceReal);
const sourceIndex = await lstat(path.join(sourceReal, 'index.html')).catch(() => null);
const destinationInfo = await lstat(destination).catch(() => null);

if (
  destination !== expectedDestination ||
  sourceReal !== expectedSource ||
  destinationParentReal !== path.join(rootReal, 'apps', 'api', 'internal', 'webui') ||
  sourceIndex === null ||
  !sourceIndex.isFile() ||
  sourceIndex.isSymbolicLink() ||
  (destinationInfo !== null &&
    (!destinationInfo.isDirectory() || destinationInfo.isSymbolicLink())) ||
  !sourceEntries.includes('index.html')
) {
  throw new Error('Production web build is missing or embed destination is invalid.');
}

await mkdir(destination, { recursive: true });
for (const entry of await readdir(destination)) {
  if (entry !== 'embed.marker') {
    await rm(path.join(destination, entry), { recursive: true, force: true });
  }
}
await cp(sourceReal, destination, { recursive: true, force: true });
