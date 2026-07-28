import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { App } from './App';

describe('App', () => {
  it('renders the Phase 1 smoke screen', () => {
    const markup = renderToStaticMarkup(<App />);

    expect(markup).toContain('Workspace is ready.');
    expect(markup).toContain('GET /healthz');
    expect(markup).toContain('<main');
  });
});
