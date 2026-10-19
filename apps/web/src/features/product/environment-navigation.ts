const environmentPattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;
const environmentRoutes = new Set(['/resources', '/search', '/events', '/topology']);

export function readEnvironment(search: string) {
  const parameters = new URLSearchParams(search);
  const value = parameters.get('environment_id') ?? parameters.get('topology_environment');
  return value !== null && environmentPattern.test(value) ? value : undefined;
}

export function environmentNavigationSearch(path: string, source?: string, environment?: string) {
  return {
    ...(source === undefined ? {} : { source }),
    ...(environment !== undefined && environmentRoutes.has(path)
      ? { environment_id: environment }
      : {}),
  };
}

export function environmentDestination(path: string) {
  return environmentRoutes.has(path) ? path : '/resources';
}
