import { useEffect, useRef, useState } from 'react';
import { useProductStream } from './product-stream';

// A burst of stream snapshots/reset notices causes at most one bounded page
// refresh per five seconds. The latest revision wins; active reads own cleanup.
export function useLiveObservation(sourceId?: string) {
  const stream = useProductStream('health', sourceId);
  const [revision, setRevision] = useState(0);
  const latest = useRef(stream.revision);
  latest.current = stream.revision;
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(() => {
    if (latest.current === revision || timer.current !== null) return;
    timer.current = setTimeout(() => {
      timer.current = null;
      setRevision(latest.current);
    }, 5000);
  }, [stream.revision, revision]);
  useEffect(
    () => () => {
      if (timer.current !== null) {
        clearTimeout(timer.current);
        timer.current = null;
      }
    },
    [sourceId],
  );
  return { revision, status: stream.status };
}
