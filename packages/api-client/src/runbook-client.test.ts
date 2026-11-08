import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from './client';
import {
  createRunbookClient,
  parseRunbookCorpus,
  parseRunbookMaterial,
  parseRunbookResults,
} from './runbook-client';

const reference = {
  corpus_id: 'ops',
  version: 1,
  digest: 'a'.repeat(64),
  document_id: 'help',
  locale: 'en' as const,
  generation: 0,
};
const hit = { reference, title: 'Reviewed help', snippet: '  inert snippet' };
const query = { query: 'private search', locale: 'en' as const, limit: 10, cursor: '' };
describe('private reviewed document transport', () => {
  it('keeps search in the body and sends an exact current reference when opening', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ hits: [hit], next_cursor: '' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json', 'X-Request-ID': 'req_test' },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            id: 'help',
            locale: 'en',
            title: 'Help',
            body: '<script>inert</script>',
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json', 'X-Request-ID': 'req_test' },
          },
        ),
      );
    const client = createRunbookClient(
      createApiClient({ fetch: fetcher, requestIdFactory: () => 'req_test' }),
    );
    const protection = { kind: 'session' as const, csrfToken: 'A'.repeat(43) };
    const result = await client.search(query, protection);
    expect(result.hits[0]?.snippet).toBe('  inert snippet');
    const call = fetcher.mock.calls[0];
    expect(call?.[0]).toBe('/api/v1/assistant/runbooks/search');
    expect(call?.[1]?.body).toBe(JSON.stringify(query));
    expect(call?.[1]?.method).toBe('POST');
    const document = await client.document(reference, protection);
    expect(document.body).toBe('<script>inert</script>');
    expect(fetcher.mock.calls[1]?.[1]?.body).toBe(JSON.stringify(reference));
  });
  it('rejects widened or malformed prose, references and pages without partial rows', () => {
    for (const value of [
      { hits: [{ ...hit, owner: 'other' }], next_cursor: '' },
      { hits: [{ ...hit, reference: { ...reference, locale: 'fa' } }], next_cursor: '' },
      { hits: [hit, hit], next_cursor: '' },
      { hits: [hit], next_cursor: 'a.b' },
      { hits: [{ ...hit, snippet: '😀'.repeat(129) }], next_cursor: '' },
      { hits: [{ ...hit, title: 'bad\u202e' }], next_cursor: '' },
    ])
      expect(() => parseRunbookResults(value, query)).toThrow();
    const corpus = {
      id: 'ops',
      version: 1,
      audience: 'all_users',
      documents: [{ id: 'help', locale: 'fa', title: 'راهنما', body: 'متن\n\tتأییدشده' }],
    };
    expect(parseRunbookCorpus(corpus).documents).toHaveLength(1);
    for (const value of [
      { ...corpus, version: Number.MAX_SAFE_INTEGER + 1 },
      { ...corpus, documents: [corpus.documents[0], corpus.documents[0]] },
      { ...corpus, documents: [{ ...corpus.documents[0], body: '\ud800' }] },
    ])
      expect(() => parseRunbookCorpus(value)).toThrow();
    expect(() =>
      parseRunbookMaterial({
        action: 'approve',
        expected_version: 1,
        generation: 0,
        digest: '',
        corpus,
      }),
    ).toThrow();
    expect(() =>
      parseRunbookMaterial({
        action: 'approve',
        expected_version: 0,
        generation: 0,
        digest: 'a'.repeat(64),
        corpus,
      }),
    ).toThrow();
  });
  it('detaches rows and rejects a late response after cancellation', async () => {
    const raw = { hits: [hit], next_cursor: '' };
    const parsed = parseRunbookResults(raw, query);
    const first = raw.hits[0];
    if (first === undefined) throw new Error('Missing fixture row');
    first.title = 'Changed';
    expect(parsed.hits[0]?.title).toBe('Reviewed help');
    const controller = new AbortController();
    const fetcher = vi.fn<typeof fetch>().mockImplementation(() => {
      controller.abort();
      return Promise.resolve(new Response(JSON.stringify(raw), {
        status: 200,
        headers: { 'Content-Type': 'application/json', 'X-Request-ID': 'req_test' },
      }));
    });
    const client = createRunbookClient(
      createApiClient({ fetch: fetcher, requestIdFactory: () => 'req_test' }),
    );
    await expect(
      client.search(query, { kind: 'session', csrfToken: 'A'.repeat(43) }, controller.signal),
    ).rejects.toMatchObject({ name: 'AbortError' });
  });
});
