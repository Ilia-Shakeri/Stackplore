export function releaseNotes(changelog, version, repository, revision) {
  const start = changelog.indexOf(`## [${version}]`);
  if (start < 0) throw new Error('Release notes missing.');
  const next = changelog.indexOf('\n## [', start + 1);
  const notes = changelog.slice(start, next < 0 ? undefined : next).trim();
  return notes.replace(
    /\]\(((?:\.\/)?docs\/[^\s)]+)\)/gu,
    (_match, path) =>
      `](https://github.com/${repository}/blob/${revision}/${path.replace(/^\.\//u, '')})`,
  );
}
