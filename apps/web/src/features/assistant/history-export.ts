import type { AssistantHistoryInput } from '@stackplore/api-client';
import type { AssistantApi, AssistantOwner } from './assistant-api';

const runID = /^ar_[0-9a-f]{64}$/u;

// A displayed page is only a selection aid. Export always obtains a fresh owned page.
export async function freshHistoryExport(
  api: AssistantApi,
  owner: AssistantOwner,
  input: AssistantHistoryInput,
  selected: readonly string[],
  signal: AbortSignal,
): Promise<string> {
  const requireCurrent = () => {
    if (signal.aborted || api.getOwner() !== owner) {
      throw new DOMException('Private export stopped.', 'AbortError');
    }
  };
  requireCurrent();
  if (
    selected.length < 1 ||
    selected.length > 10 ||
    new Set(selected).size !== selected.length ||
    selected.some((id) => !runID.test(id))
  )
    throw new Error('Invalid metadata selection.');
  const wanted = new Set(selected);
  const page = await api.history(owner, input, signal);
  requireCurrent();
  const entries = page.entries.filter((entry) => wanted.has(entry.id));
  if (
    entries.length !== wanted.size ||
    new Set(entries.map((entry) => entry.id)).size !== wanted.size
  ) {
    throw new Error('Metadata selection changed.');
  }
  // Keep this projection closed when transport types gain fields in later versions.
  const content = JSON.stringify(
    {
      schema_version: 1,
      exported_at: new Date().toISOString(),
      entries: entries.map((entry) => ({
        id: entry.id,
        source_id: entry.source_id,
        resource_id: entry.resource_id,
        goal: entry.goal,
        state: entry.state,
        created_at: entry.created_at,
        updated_at: entry.updated_at,
        range: { from: entry.range.from, to: entry.range.to },
        tool_calls: entry.tool_calls,
        inference_calls: entry.inference_calls,
        usage_uncertain: entry.usage_uncertain,
      })),
    },
    null,
    2,
  );
  if (new TextEncoder().encode(content).length > 32 * 1024) {
    throw new Error('Metadata export is too large.');
  }
  requireCurrent();
  return content;
}
