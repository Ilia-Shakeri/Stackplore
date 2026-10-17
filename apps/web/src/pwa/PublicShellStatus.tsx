import { Button } from '@stackplore/ui';
import { useEffect, useRef, useState } from 'react';

import { useLocale } from '../i18n';
import { registerPublicShell, type PublicShellRegistration } from './register';
import en from './messages.en.json';
import fa from './messages.fa.json';
import './status.css';

const messages = { en, fa };

export function PublicShellStatus() {
  const { locale } = useLocale();
  const text = messages[locale];
  const [offline, setOffline] = useState(!navigator.onLine);
  const [update, setUpdate] = useState(false);
  const registration = useRef<PublicShellRegistration | undefined>(undefined);

  useEffect(() => {
    const onConnection = () => {
      setOffline(!navigator.onLine);
    };
    window.addEventListener('online', onConnection);
    window.addEventListener('offline', onConnection);
    let disposed = false;
    if (import.meta.env.PROD) {
      void registerPublicShell(() => {
        setUpdate(true);
      })
        .then((value) => {
          if (disposed) value?.dispose();
          else registration.current = value;
        })
        .catch(() => {
          // The online app remains usable when offline support cannot install.
        });
    }
    return () => {
      disposed = true;
      registration.current?.dispose();
      window.removeEventListener('online', onConnection);
      window.removeEventListener('offline', onConnection);
    };
  }, []);

  if (!offline && !update) return null;
  return (
    <aside className="public-shell-status" aria-label={text.label}>
      <p role="status" aria-live="polite">
        {offline ? text.offline : text.update}
      </p>
      {update && !offline ? (
        <Button size="sm" onClick={() => registration.current?.activateUpdate()}>
          {text.reload}
        </Button>
      ) : null}
    </aside>
  );
}
