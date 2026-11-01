import { parseAssistantRunResponse, type AssistantIncidentReportResponse } from '@stackplore/api-client';
import { afterEach, describe, expect, it } from 'vitest';
import { AssistantConversation } from './conversation';
import { controlledConversationApi, investigationResponse, incidentResponse, scope } from './conversation.test-fixtures';

const owned: AssistantConversation[] = [];
async function make(large = false) {
  const controlled = controlledConversationApi();
  let ready = investigationResponse();
  if (ready.content_state !== 'ready') throw new Error('Missing report fixture');
  if (large) {
    const sections = [ready.answer.sections[0], ...[4096, 4096, 1].map((length) => ({ kind: 'gaps', text: '\x01'.repeat(length), citations: [] }))];
    const last = sections[3];
    if (last === undefined) throw new Error('Missing bounded budget fixture');
    const occupied = new TextEncoder().encode(JSON.stringify({ sections })).byteLength;
    last.text = '\x01'.repeat(Math.floor((65536 - occupied - 128) / 6));
    ready = parseAssistantRunResponse({ ...ready, answer: { sections } }, ready.run.id);
  }
  controlled.api.admit.mockResolvedValue({ run: ready.run });
  controlled.api.get.mockResolvedValue(ready);
  controlled.api.report.mockResolvedValue(incidentResponse());
  const conversation = new AssistantConversation(controlled.api, { ...scope, goal: 'investigate_incident' });
  owned.push(conversation); conversation.connect();
  conversation.setDraft('Investigate selected observations.');
  await conversation.submit();
  return { ...controlled, conversation, ready };
}
afterEach(() => { for (const conversation of owned.splice(0)) conversation.dispose(); });

describe('volatile incident report ownership', () => {
  it('reads only on an explicit request without another admission', async () => {
    const { api, conversation, ready } = await make();
    expect(api.report).not.toHaveBeenCalled();
    const result = await conversation.resolveReport(ready.run.id);
    expect(result?.content_state).toBe('ready');
    expect(api.report).toHaveBeenCalledOnce();
    expect(api.admit).toHaveBeenCalledOnce();
  });

  it('aborts on owner loss and rejects a late report from a noncooperating port', async () => {
    const { api, conversation, ready, setOwner } = await make();
    let settle: (value: AssistantIncidentReportResponse) => void = () => undefined;
    api.report.mockImplementation(() => new Promise((resolve) => { settle = resolve; }));
    const pending = conversation.resolveReport(ready.run.id);
    const signal = api.report.mock.calls[0]?.[2];
    setOwner(Symbol());
    expect(signal?.aborted).toBe(true);
    settle(incidentResponse());
    expect(await pending).toBeNull();
    expect(conversation.getSnapshot().turns).toHaveLength(0);
  });

  it('closing a report aborts its request and keeps the existing answer', async () => {
    const { api, conversation, ready } = await make();
    let settle: (value: AssistantIncidentReportResponse) => void = () => undefined;
    api.report.mockImplementation(() => new Promise((resolve) => { settle = resolve; }));
    const controller = new AbortController();
    const pending = conversation.resolveReport(ready.run.id, controller.signal);
    controller.abort(); settle(incidentResponse());
    expect(await pending).toBeNull();
    expect(conversation.getSnapshot().turns).toHaveLength(1);
    expect(api.admit).toHaveBeenCalledOnce();
  });

  it('rejects changed retained citation time and the second output allowance', async () => {
    const { api, conversation, ready } = await make();
    const reply = incidentResponse();
    if (reply.content_state !== 'ready') throw new Error('Missing ready report');
    api.report.mockResolvedValue({ ...reply, citations: reply.citations.map((ref) => ({ ...ref, observed_at: '2026-10-28T10:15:02Z' })) });
    expect(await conversation.resolveReport(ready.run.id)).toBeNull();
    const oversized = await make(true);
    expect(await oversized.conversation.resolveReport(oversized.ready.run.id)).toBeNull();
  });

  it('clears all retained material when the server confirms lost memory', async () => {
    const { api, conversation, ready } = await make();
    const reply = incidentResponse();
    api.report.mockResolvedValue({ run: reply.run, content_state: 'volatile_lost' } as AssistantIncidentReportResponse);
    expect(await conversation.resolveReport(ready.run.id)).toBeNull();
    expect(conversation.getSnapshot()).toMatchObject({ state: 'unavailable', turns: [], draft: '' });
  });
});
