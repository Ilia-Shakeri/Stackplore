import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';

import type { ProductApi } from './product-api';

interface ProductContextValue {
  readonly api: ProductApi;
  readonly selectedSource?: string;
  readonly selectSource: (sourceId: string) => void;
  readonly query: string;
  readonly selectQuery: (query: string) => void;
}

const ProductContext = createContext<ProductContextValue | null>(null);
const sourcePattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;

export function readProductSource(search: string) {
  const value = new URLSearchParams(search).get('source');
  return value !== null && sourcePattern.test(value) ? value : undefined;
}

export function readProductQuery(search: string) {
  const query = new URLSearchParams(search).get('q') ?? '';
  return query.length <= 200 && query.trim() === query ? query : '';
}

function initialQuery() {
  return typeof window === 'undefined' ? '' : readProductQuery(window.location.search);
}

function initialSource() {
  if (typeof window === 'undefined') return undefined;
  return readProductSource(window.location.search);
}

export function ProductProvider({
  api,
  children,
  onSourceChange,
  sourceId,
  onQueryChange,
  query: controlledQuery,
}: {
  readonly api: ProductApi;
  readonly children: ReactNode;
  readonly onSourceChange?: (sourceId: string) => void;
  readonly sourceId?: string;
  readonly onQueryChange?: (query: string) => void;
  readonly query?: string;
}) {
  const [localSource, setLocalSource] = useState<string | undefined>(initialSource);
  const selectedSource = onSourceChange === undefined ? localSource : sourceId;
  const [localQuery, setLocalQuery] = useState(initialQuery);
  const query = onQueryChange === undefined ? localQuery : (controlledQuery ?? '');
  useEffect(() => {
    if (onSourceChange !== undefined) return;
    const syncSource = () => {
      setLocalSource(initialSource());
      setLocalQuery(initialQuery());
    };
    window.addEventListener('popstate', syncSource);
    return () => {
      window.removeEventListener('popstate', syncSource);
    };
  }, [onSourceChange]);
  const selectSource = useCallback(
    (sourceId: string) => {
      if (!sourcePattern.test(sourceId)) throw new Error('Source identifier is invalid.');
      if (onSourceChange !== undefined) {
        onSourceChange(sourceId);
        return;
      }
      setLocalSource(sourceId);
      if (typeof window !== 'undefined') {
        const url = new URL(window.location.href);
        url.searchParams.set('source', sourceId);
        window.history.replaceState(window.history.state, '', url);
      }
    },
    [onSourceChange],
  );
  const selectQuery = useCallback(
    (value: string) => {
      if (value.length > 200 || value.trim() !== value) throw new Error('Search query is invalid.');
      if (onQueryChange !== undefined) {
        onQueryChange(value);
        return;
      }
      setLocalQuery(value);
      if (typeof window !== 'undefined') {
        const url = new URL(window.location.href);
        if (value === '') url.searchParams.delete('q');
        else url.searchParams.set('q', value);
        window.history.replaceState(window.history.state, '', url);
      }
    },
    [onQueryChange],
  );
  const value = useMemo(
    () => ({
      api,
      query,
      selectQuery,
      selectSource,
      ...(selectedSource === undefined ? {} : { selectedSource }),
    }),
    [api, query, selectQuery, selectSource, selectedSource],
  );
  return <ProductContext value={value}>{children}</ProductContext>;
}

export function useProductApi() {
  const value = useContext(ProductContext);
  if (value === null) throw new Error('useProductApi must be used within ProductProvider.');
  return value.api;
}

export function useProductSource() {
  const value = useContext(ProductContext);
  if (value === null) throw new Error('useProductSource must be used within ProductProvider.');
  return {
    selectedSource: value.selectedSource,
    selectSource: value.selectSource,
    query: value.query,
    selectQuery: value.selectQuery,
  };
}
