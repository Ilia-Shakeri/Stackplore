export interface PublicShellRegistration {
  dispose: () => void;
  activateUpdate: () => boolean;
}

export async function registerPublicShell(
  onUpdate: () => void,
  container: ServiceWorkerContainer | null = 'serviceWorker' in navigator
    ? navigator.serviceWorker
    : null,
  reload: () => void = () => {
    window.location.reload();
  },
): Promise<PublicShellRegistration | undefined> {
  if (!container || !window.isSecureContext) return undefined;
  const registration = await container.register('/sw.js', { scope: '/', updateViaCache: 'none' });
  let requested = false;
  let disposed = false;
  let installing: ServiceWorker | null = null;
  const stateChanged = () => {
    if (!disposed && installing?.state === 'installed' && container.controller) onUpdate();
  };
  const updateFound = () => {
    installing?.removeEventListener('statechange', stateChanged);
    installing = registration.installing;
    installing?.addEventListener('statechange', stateChanged);
  };
  const controllerChanged = () => {
    if (requested && !disposed) {
      requested = false;
      reload();
    }
  };
  registration.addEventListener('updatefound', updateFound);
  container.addEventListener('controllerchange', controllerChanged);
  updateFound();
  if (registration.waiting && container.controller) onUpdate();
  return {
    activateUpdate: () => {
      const waiting =
        registration.waiting ?? (installing?.state === 'installed' ? installing : null);
      if (disposed || !waiting) return false;
      requested = true;
      waiting.postMessage({ type: 'ACTIVATE_PUBLIC_SHELL' });
      return true;
    },
    dispose: () => {
      disposed = true;
      installing?.removeEventListener('statechange', stateChanged);
      registration.removeEventListener('updatefound', updateFound);
      container.removeEventListener('controllerchange', controllerChanged);
    },
  };
}
