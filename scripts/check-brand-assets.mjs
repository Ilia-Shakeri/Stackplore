import { createHash } from 'node:crypto';
import { lstat, readFile, readdir } from 'node:fs/promises';
import process from 'node:process';
import { URL } from 'node:url';

const directory = new URL('../apps/web/public/brand/', import.meta.url);
const manifest = JSON.parse(await readFile(new URL('assets.json', directory), 'utf8'));
const fail = () => {
  throw new Error('Brand asset inventory, dimensions or byte budget changed.');
};
const specifications = new Map([
  ['stackplore-mark-96.webp', [96, 96]],
  ['stackplore-mark-192.webp', [192, 192]],
  ['stackplore-mark-512.webp', [512, 512]],
  ['stackplore-favicon-32.png', [32, 32]],
  ['stackplore-favicon-64.png', [64, 64]],
  ['stackplore-apple-touch.png', [180, 180]],
  ['stackplore-pwa-192.png', [192, 192]],
  ['stackplore-pwa-512.png', [512, 512]],
  ['stackplore-maskable-512.png', [512, 512]],
  ['stackplore-intro-dark.webp', [960, 640]],
  ['stackplore-intro-light.webp', [960, 640]],
  ['stackplore-social-preview.jpg', [1200, 630]],
]);
if (
  !Array.isArray(manifest.outputs) ||
  manifest.outputs.length !== specifications.size ||
  !Array.isArray(manifest.sources) ||
  manifest.sources.length !== 3
)
  fail();
const files = await readdir(directory);
if (
  files.length !== specifications.size + 1 ||
  files.some((name) => name !== 'assets.json' && !specifications.has(name))
)
  fail();
const seen = new Set();
let bytes = 0;
for (const entry of manifest.outputs) {
  const expected = specifications.get(entry.name);
  if (
    !expected ||
    seen.has(entry.name) ||
    entry.width !== expected[0] ||
    entry.height !== expected[1] ||
    !Number.isSafeInteger(entry.bytes) ||
    entry.bytes < 1 ||
    entry.bytes > 200 * 1024 ||
    !/^[a-f0-9]{64}$/u.test(entry.sha256)
  )
    fail();
  seen.add(entry.name);
  const location = new URL(entry.name, directory);
  const info = await lstat(location);
  if (!info.isFile() || info.isSymbolicLink() || info.size !== entry.bytes) fail();
  const data = await readFile(location);
  if (createHash('sha256').update(data).digest('hex') !== entry.sha256) fail();
  if (entry.name.endsWith('.png')) {
    if (
      data.length < 33 ||
      data.subarray(0, 8).toString('hex') !== '89504e470d0a1a0a' ||
      data.toString('ascii', 12, 16) !== 'IHDR' ||
      data.readUInt32BE(16) !== entry.width ||
      data.readUInt32BE(20) !== entry.height
    )
      fail();
  } else if (entry.name.endsWith('.webp')) {
    if (
      data.length < 30 ||
      data.toString('ascii', 0, 4) !== 'RIFF' ||
      data.toString('ascii', 8, 12) !== 'WEBP' ||
      data.readUInt32LE(4) + 8 !== data.length
    )
      fail();
    const kind = data.toString('ascii', 12, 16);
    const dimensions =
      kind === 'VP8X'
        ? [data.readUIntLE(24, 3) + 1, data.readUIntLE(27, 3) + 1]
        : kind === 'VP8 ' && data.toString('hex', 23, 26) === '9d012a'
          ? [data.readUInt16LE(26) & 0x3fff, data.readUInt16LE(28) & 0x3fff]
          : [];
    if (dimensions[0] !== entry.width || dimensions[1] !== entry.height) fail();
  } else {
    if (data.toString('hex', 0, 2) !== 'ffd8' || data.toString('hex', data.length - 2) !== 'ffd9')
      fail();
    let dimensions;
    for (let offset = 2; offset + 4 < data.length;) {
      if (data[offset] !== 0xff) fail();
      const marker = data[offset + 1];
      if (marker === 0xda || marker === 0xd9) break;
      const length = data.readUInt16BE(offset + 2);
      if (length < 2 || offset + 2 + length > data.length) fail();
      if ([0xc0, 0xc1, 0xc2].includes(marker)) {
        if (length < 8) fail();
        dimensions = [data.readUInt16BE(offset + 7), data.readUInt16BE(offset + 5)];
        break;
      }
      offset += 2 + length;
    }
    if (dimensions?.[0] !== entry.width || dimensions[1] !== entry.height) fail();
  }
  bytes += info.size;
}
const sourceBytes = manifest.sources.reduce((total, source) => {
  if (
    !Number.isSafeInteger(source.bytes) ||
    source.bytes <= 0 ||
    !/^[a-f0-9]{64}$/u.test(source.sha256)
  )
    fail();
  return total + source.bytes;
}, 0);
if (
  bytes !== manifest.optimized_total_bytes ||
  sourceBytes !== manifest.original_total_bytes ||
  bytes > 600 * 1024 ||
  bytes / sourceBytes > 0.2
)
  fail();
process.stdout.write(
  `Brand assets pass: ${seen.size} files, ${bytes} bytes, ${(100 * (1 - bytes / sourceBytes)).toFixed(1)}% smaller than the three sources.\n`,
);
