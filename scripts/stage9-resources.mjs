import { readFile, readdir } from 'node:fs/promises';

export function processResources(status, descriptors) {
  const resident = /^VmRSS:\s+([0-9]+)\s+kB$/mu.exec(status)?.[1];
  const peak = /^VmHWM:\s+([0-9]+)\s+kB$/mu.exec(status)?.[1];
  const rss = Number(resident) * 1024;
  const high = Number(peak) * 1024;
  if (!resident || !peak || !Number.isSafeInteger(rss) || rss < 1 || !Number.isSafeInteger(high) || high < rss || !Number.isInteger(descriptors) || descriptors < 1 || descriptors > 4096)
    throw new Error('stage9_resources.invalid');
  return { rss_bytes: rss, peak_rss_bytes: high, descriptors };
}

export async function readOwnedResources(pid) {
  if (!Number.isSafeInteger(pid) || pid < 1) throw new Error('stage9_resources.pid');
  const [status, descriptors] = await Promise.all([readFile(`/proc/${pid}/status`, 'utf8'), readdir(`/proc/${pid}/fd`)]);
  return processResources(status, descriptors.length);
}

export function validateResourceSamples(samples, requireHour) {
  if (!Array.isArray(samples) || samples.length < (requireHour ? 120 : 1) || samples.length > 180)
    throw new Error('stage9_resources.samples');
  let previous = -1;
  for (const sample of samples) {
    if (!Number.isSafeInteger(sample.elapsed_ms) || sample.elapsed_ms <= previous) throw new Error('stage9_resources.order');
    previous = sample.elapsed_ms;
    for (const [name, ceiling] of [['application', 512 * 1024 * 1024], ['processor', 8 * 1024 * 1024 * 1024]]) {
      const value = sample[name];
      if (!Number.isSafeInteger(value?.rss_bytes) || value.rss_bytes < 1 || !Number.isSafeInteger(value.peak_rss_bytes) || value.peak_rss_bytes < value.rss_bytes || value.peak_rss_bytes > ceiling || !Number.isInteger(value.descriptors) || value.descriptors < 1 || value.descriptors > 256)
        throw new Error('stage9_resources.budget');
    }
  }
  if (requireHour && samples.at(-1).elapsed_ms - samples[0].elapsed_ms < 3600000) throw new Error('stage9_resources.duration');
  // Compare settled windows; an exact zero-growth claim is not supported by RSS.
  if (requireHour) {
    const first = samples.slice(10, 20), last = samples.slice(-10);
    for (const name of ['application', 'processor']) {
      const baseline = Math.max(...first.map((s) => s[name].rss_bytes));
      const final = Math.max(...last.map((s) => s[name].rss_bytes));
      const descriptors = Math.max(...last.map((s) => s[name].descriptors)) - Math.max(...first.map((s) => s[name].descriptors));
      if (final - baseline > 64 * 1024 * 1024 || descriptors > 32) throw new Error('stage9_resources.growth');
    }
  }
  return { status: 'passed', samples: samples.length, last_elapsed_ms: previous, scope: 'Owned application and processor RSS/descriptor observations; bounded growth, not proof of zero leaks or customer capacity' };
}
