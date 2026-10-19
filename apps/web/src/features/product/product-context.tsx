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
import { hasCodePointLength } from '@stackplore/api-client';

interface ProductContextValue {
  readonly api: ProductApi;
  readonly selectedSource?: string;
  readonly selectSource: (sourceId: string) => void;
  readonly query: string;
  readonly selectQuery: (query: string) => void;
  readonly viewSearch: string;
  readonly selectViewParameter: (name: string, value: string) => void;
}

const ProductContext = createContext<ProductContextValue | null>(null);
const sourcePattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;

export function readProductSource(search: string) {
  const value = new URLSearchParams(search).get('source');
  return value !== null && sourcePattern.test(value) ? value : undefined;
}

export function readProductQuery(search: string) {
  const query = new URLSearchParams(search).get('q') ?? '';
  return hasCodePointLength(query, 200) && query.trim() === query ? query : '';
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
  viewSearch,
  onViewParameterChange,
}: {
  readonly api: ProductApi;
  readonly children: ReactNode;
  readonly onSourceChange?: (sourceId: string) => void;
  readonly sourceId?: string;
  readonly onQueryChange?: (query: string) => void;
  readonly query?: string;
  readonly viewSearch?: string;
  readonly onViewParameterChange?: (name: string, value: string) => void;
}) {
  const [localViewSearch, setLocalViewSearch] = useState(() =>
    typeof window === 'undefined' ? '' : window.location.search,
  );
  const [localSource, setLocalSource] = useState<string | undefined>(initialSource);
  const selectedSource = onSourceChange === undefined ? localSource : sourceId;
  const [localQuery, setLocalQuery] = useState(initialQuery);
  const query = onQueryChange === undefined ? localQuery : (controlledQuery ?? '');
  useEffect(() => {
    const syncSource = () => {
      if (onSourceChange === undefined) setLocalSource(initialSource());
      if (onQueryChange === undefined) setLocalQuery(initialQuery());
      setLocalViewSearch(window.location.search);
    };
    window.addEventListener('popstate', syncSource);
    return () => {
      window.removeEventListener('popstate', syncSource);
    };
  }, [onSourceChange, onQueryChange]);
  const selectSource = useCallback(
    (sourceId: string) => {
      if (!sourcePattern.test(sourceId)) throw new Error('Source identifier is invalid.');
      if (sourceId === selectedSource) return;
      if (onSourceChange !== undefined) {
        onSourceChange(sourceId);
        return;
      }
      setLocalSource(sourceId);
      if (typeof window !== 'undefined') {
        const url = new URL(window.location.href);
        url.searchParams.set('source', sourceId);
        for (const key of [
          'environment_id',
          'topology_environment',
          'topology_selected',
          'resource_id',
          'container',
          'rule_cursor',
        ])
          url.searchParams.delete(key);
        window.history.replaceState(window.history.state, '', url);
        setLocalViewSearch(url.search);
      }
    },
    [onSourceChange, selectedSource],
  );
  const selectQuery = useCallback(
    (value: string) => {
      if (!hasCodePointLength(value, 200) || value.trim() !== value)
        throw new Error('Search query is invalid.');
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
  const selectViewParameter = useCallback(
    (name: string, value: string) => {
      if (onViewParameterChange !== undefined) {
        onViewParameterChange(name, value);
        return;
      }
      if (typeof window === 'undefined') return;
      const url = new URL(window.location.href);
      if (value === '') url.searchParams.delete(name);
      else url.searchParams.set(name, value);
      if (name === 'environment_id' || name === 'topology_environment') {
        url.searchParams.delete('topology_environment');
        if (value === '') url.searchParams.delete('environment_id');
        else url.searchParams.set('environment_id', value);
        for (const key of ['topology_selected', 'resource_id', 'container', 'rule_cursor'])
          url.searchParams.delete(key);
      }
      window.history.replaceState(window.history.state, '', url);
      setLocalViewSearch(url.search);
    },
    [onViewParameterChange],
  );
  const value = useMemo(
    () => ({
      api,
      query,
      selectQuery,
      selectSource,
      viewSearch: viewSearch ?? localViewSearch,
      selectViewParameter,
      ...(selectedSource === undefined ? {} : { selectedSource }),
    }),
    [
      api,
      query,
      selectQuery,
      selectSource,
      selectedSource,
      viewSearch,
      localViewSearch,
      selectViewParameter,
    ],
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

export function useProductView() {
  const value = useContext(ProductContext);
  const [fallbackSearch, setFallbackSearch] = useState(() =>
    typeof window === 'undefined' ? '' : window.location.search,
  );
  useEffect(() => {
    if (value !== null) return;
    const sync = () => {
      setFallbackSearch(window.location.search);
    };
    window.addEventListener('popstate', sync);
    return () => {
      window.removeEventListener('popstate', sync);
    };
  }, [value]);
  const fallbackWrite = useCallback((name: string, entry: string) => {
    if (typeof window === 'undefined') return;
    const url = new URL(window.location.href);
    if (entry === '') url.searchParams.delete(name);
    else url.searchParams.set(name, entry);
    if (name === 'environment_id' || name === 'topology_environment') {
      url.searchParams.delete('topology_environment');
      if (entry === '') url.searchParams.delete('environment_id');
      else url.searchParams.set('environment_id', entry);
      for (const key of ['topology_selected', 'resource_id', 'container', 'rule_cursor'])
        url.searchParams.delete(key);
    }
    window.history.replaceState(window.history.state, '', url);
    setFallbackSearch(url.search);
  }, []);
  return {
    search: value?.viewSearch ?? fallbackSearch,
    selectParameter: value?.selectViewParameter ?? fallbackWrite,
  };
}
