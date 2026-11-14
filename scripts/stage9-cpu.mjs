function allowedCPUs(status) {
  const value = /^Cpus_allowed_list:\s+([0-9,-]{1,128})$/mu.exec(status)?.[1];
  if (!value) throw new Error('stage9_cpu.affinity');
  const cpus = [];
  for (const part of value.split(',')) {
    const match = /^(0|[1-9][0-9]{0,3})(?:-(0|[1-9][0-9]{0,3}))?$/u.exec(part);
    if (!match) throw new Error('stage9_cpu.affinity');
    const start = Number(match[1]), end = Number(match[2] ?? match[1]);
    if (end < start || end - start > 3) throw new Error('stage9_cpu.affinity');
    for (let id = start; id <= end; id++) {
      if (cpus.includes(id) || cpus.length === 4) throw new Error('stage9_cpu.affinity');
      cpus.push(id);
    }
  }
  cpus.sort((a, b) => a - b);
  return cpus;
}

export function workloadCPUSet(status) {
  const cpus = allowedCPUs(status);
  if (cpus.length !== 4) throw new Error('stage9_cpu.four_cpus_required');
  return { processor: cpus.slice(0, 3).join(','), application_database_browser: String(cpus[3]) };
}

export function assertCPUSet(status, expected) {
  if (allowedCPUs(status).join(',') !== expected) throw new Error('stage9_cpu.ownership');
}
