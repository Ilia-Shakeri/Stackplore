import { Buffer } from 'node:buffer';
import { createServer, request } from 'node:http';
import { performance } from 'node:perf_hooks';

const maximumBytes = 1024 * 1024;
const methods = new Map([['/v1/models', 'GET'], ['/props', 'GET'], ['/apply-template', 'POST'], ['/tokenize', 'POST'], ['/completion', 'POST']]);

export function assertHostedObserverEnvironment(environment) {
  if (environment.GITHUB_ACTIONS !== 'true' || environment.RUNNER_ENVIRONMENT !== 'github-hosted' || !/^[a-f0-9]{40}$/u.test(environment.GITHUB_SHA ?? '')) {
    throw new Error('hosted_processor.observer_runner_required');
  }
}

// Observe counts only; every response byte still comes from the real pinned engine.
export async function startHostedProcessorObserver(environment) {
  assertHostedObserverEnvironment(environment);
  const calls = [];
  const server = createServer(async (incoming, outgoing) => {
    if (!methods.has(incoming.url) || methods.get(incoming.url) !== incoming.method) { outgoing.writeHead(404).end(); return; }
    const chunks = [];
    let bytes = 0;
    let body;
    try {
      for await (const chunk of incoming) {
        bytes += chunk.length;
        if (bytes > maximumBytes) throw new Error('bounded request');
        chunks.push(chunk);
      }
      body = Buffer.concat(chunks);
      for (const chunk of chunks) chunk.fill(0);
      const started = performance.now();
      let observation;
      if (incoming.url === '/completion') {
        if (calls.length >= 8) { body.fill(0); outgoing.writeHead(429).end(); return; }
        const input = JSON.parse(body.toString('utf8'));
        observation = { input_tokens: Array.isArray(input.prompt) ? input.prompt.length : 0, output_limit: Number.isInteger(input.n_predict) ? input.n_predict : 0, elapsed_ms: 0, http_status: 0, fault: 'unfinished', output_tokens: 0, complete: false, input_match: false, sections: 0, citation_count: 0 };
        calls.push(observation);
      }
      const upstream = request({ hostname: '127.0.0.1', port: 58102, path: incoming.url, method: incoming.method, agent: false, timeout: 45_000, headers: { Accept: 'application/json', 'Content-Type': 'application/json' } }, (response) => {
        const captured = [];
        let size = 0;
        if (observation) observation.http_status = response.statusCode ?? 0;
        outgoing.writeHead(response.statusCode ?? 502, { 'Content-Type': 'application/json' });
        response.on('data', (chunk) => {
          size += chunk.length;
          if (size > maximumBytes) { response.destroy(); upstream.destroy(); return; }
          if (observation) captured.push(Buffer.from(chunk));
          outgoing.write(chunk);
        });
        response.on('end', () => {
          if (observation) {
            observation.elapsed_ms = Math.round(performance.now() - started);
            observation.fault = response.statusCode === 200 ? 'none' : 'http_error';
            const contents = Buffer.concat(captured);
            try {
              const result = JSON.parse(contents.toString('utf8'));
              observation.output_tokens = Number.isInteger(result.tokens_predicted) ? result.tokens_predicted : 0;
              observation.input_match = result.tokens_evaluated === observation.input_tokens;
              observation.complete = result.truncated === false && ['eos', 'word'].includes(result.stop_type);
              const answer = typeof result.content === 'string' ? JSON.parse(result.content) : null;
              if (Array.isArray(answer?.sections) && answer.sections.length <= 12) {
                observation.sections = answer.sections.length;
                observation.citation_count = answer.sections.reduce((sum, section) => sum + (Array.isArray(section.citations) ? Math.min(section.citations.length, 64) : 0), 0);
              }
            } catch { observation.fault = 'shape_unavailable'; }
            contents.fill(0);
            for (const chunk of captured) chunk.fill(0);
          }
          outgoing.end();
        });
        response.on('error', () => {
          for (const chunk of captured) chunk.fill(0);
          if (observation && observation.fault === 'unfinished') { observation.elapsed_ms = Math.round(performance.now() - started); observation.fault = 'upstream_closed'; }
          outgoing.destroy();
        });
      });
      upstream.on('timeout', () => upstream.destroy());
      upstream.on('error', () => {
        body.fill(0);
        if (observation && observation.fault === 'unfinished') { observation.elapsed_ms = Math.round(performance.now() - started); observation.fault = 'upstream_error'; }
        if (!outgoing.headersSent) outgoing.writeHead(502);
        outgoing.end();
      });
      outgoing.on('close', () => {
        if (!outgoing.writableFinished && observation) { observation.elapsed_ms = Math.round(performance.now() - started); observation.fault = 'client_closed'; }
        upstream.destroy();
      });
      upstream.end(body, () => body.fill(0));
    } catch {
      body?.fill(0);
      for (const chunk of chunks) chunk.fill(0);
      if (!outgoing.headersSent) outgoing.writeHead(413);
      outgoing.end();
    }
  });
  server.maxConnections = 2;
  server.requestTimeout = 45_000;
  server.keepAliveTimeout = 1000;
  await new Promise((accept, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', accept); });
  const address = server.address();
  return {
    origin: `http://127.0.0.1:${address.port}`,
    snapshot: () => globalThis.structuredClone(calls),
    async stop() { server.closeAllConnections(); await new Promise((accept) => server.close(accept)); },
  };
}
