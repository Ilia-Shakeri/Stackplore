import { useEffect, useState } from 'react';

export type ProductStreamStatus =
  'connecting' | 'connected' | 'stale' | 'disconnected' | 'unavailable';

export type ProductStreamTopic = 'alerts' | 'events' | 'health' | 'logs' | 'metrics';

interface ProductStreamState {
  readonly revision: number;
  readonly status: ProductStreamStatus;
}

const sourcePattern = /^[A-Za-z][A-Za-z0-9_-]{0,127}$/u;

export function useProductStream(topic: ProductStreamTopic, sourceId?: string): ProductStreamState {
  const [state, setState] = useState<ProductStreamState>({ revision: 0, status: 'connecting' });

  useEffect(() => {
    if (typeof EventSource === 'undefined') {
      setState({ revision: 0, status: 'unavailable' });
      return;
    }
    const query =
      sourceId === undefined || !sourcePattern.test(sourceId)
        ? ''
        : `?source_id=${encodeURIComponent(sourceId)}`;
    const stream = new EventSource(`/api/v1/streams/${topic}${query}`, { withCredentials: true });
    const snapshotEvent = `${topic}.snapshot.v1`;
    const onOpen = () => {
      setState((current) => ({ ...current, status: 'connected' }));
    };
    const onSnapshot = () => {
      setState((current) => ({ revision: current.revision + 1, status: 'connected' }));
    };
    const onReset = () => {
      setState((current) => ({ ...current, status: 'stale' }));
    };
    const onError = () => {
      setState((current) => ({ ...current, status: 'disconnected' }));
    };
    stream.addEventListener('open', onOpen);
    stream.addEventListener(snapshotEvent, onSnapshot);
    stream.addEventListener('stream.reset.v1', onReset);
    stream.addEventListener('error', onError);
    return () => {
      stream.removeEventListener('open', onOpen);
      stream.removeEventListener(snapshotEvent, onSnapshot);
      stream.removeEventListener('stream.reset.v1', onReset);
      stream.removeEventListener('error', onError);
      stream.close();
    };
  }, [sourceId, topic]);

  return state;
}
