import * as ToastPrimitive from '@radix-ui/react-toast';
import { X } from 'lucide-react';
import {
  createContext,
  useCallback,
  useContext,
  useId,
  useMemo,
  useRef,
  useState,
  type ComponentPropsWithoutRef,
  type ReactNode,
} from 'react';

import { cn } from '../lib/cn';

const TOAST_LIMIT = 3;

export type ToastTone = 'info' | 'success' | 'warning' | 'danger';

export interface ToastAction {
  label: string;
  onClick: () => void;
}

export interface ToastOptions {
  action?: ToastAction;
  description?: ReactNode;
  duration?: number;
  title: ReactNode;
  tone?: ToastTone;
}

interface ToastItem {
  action?: ToastAction;
  description?: ReactNode;
  duration: number;
  id: string;
  title: ReactNode;
  tone: ToastTone;
}

interface ToastContextValue {
  dismiss: (id: string) => void;
  notify: (options: ToastOptions) => string;
}

export interface ToastProviderProps extends Omit<
  ComponentPropsWithoutRef<typeof ToastPrimitive.Provider>,
  'children' | 'duration'
> {
  children: ReactNode;
  closeLabel: string;
  duration?: number;
  viewportLabel: string;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export function ToastProvider({
  children,
  closeLabel,
  duration = 5000,
  swipeDirection = 'right',
  viewportLabel,
  ...props
}: ToastProviderProps) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const prefix = useId();
  const sequence = useRef(0);

  const dismiss = useCallback((id: string) => {
    setItems((current) => current.filter((item) => item.id !== id));
  }, []);

  const notify = useCallback(
    (options: ToastOptions) => {
      sequence.current += 1;
      const id = `${prefix}-${String(sequence.current)}`;
      const item: ToastItem = {
        duration: options.duration ?? duration,
        id,
        title: options.title,
        tone: options.tone ?? 'info',
        ...(options.action === undefined ? {} : { action: options.action }),
        ...(options.description === undefined ? {} : { description: options.description }),
      };

      setItems((current) => [...current.slice(-(TOAST_LIMIT - 1)), item]);
      return id;
    },
    [duration, prefix],
  );

  const value = useMemo(() => ({ dismiss, notify }), [dismiss, notify]);

  return (
    <ToastContext value={value}>
      <ToastPrimitive.Provider swipeDirection={swipeDirection} {...props}>
        {children}
        {items.map((item) => (
          <ToastPrimitive.Root
            key={item.id}
            className="ui-toast"
            data-tone={item.tone}
            duration={item.duration}
            open
            onOpenChange={(open) => {
              if (!open) {
                dismiss(item.id);
              }
            }}
          >
            <ToastPrimitive.Title className="ui-toast__title">{item.title}</ToastPrimitive.Title>
            {item.description === undefined ? null : (
              <ToastPrimitive.Description className="ui-toast__description">
                {item.description}
              </ToastPrimitive.Description>
            )}
            {item.action === undefined ? null : (
              <ToastPrimitive.Action
                asChild
                className="ui-toast__action ui-focus-ring"
                altText={item.action.label}
              >
                <button type="button" onClick={item.action.onClick}>
                  {item.action.label}
                </button>
              </ToastPrimitive.Action>
            )}
            <ToastPrimitive.Close className="ui-toast__close ui-focus-ring" aria-label={closeLabel}>
              <X aria-hidden="true" />
            </ToastPrimitive.Close>
          </ToastPrimitive.Root>
        ))}
        <ToastPrimitive.Viewport
          className={cn('ui-toast__viewport')}
          data-swipe-direction={swipeDirection}
          label={viewportLabel}
        />
      </ToastPrimitive.Provider>
    </ToastContext>
  );
}

export function useToast() {
  const value = useContext(ToastContext);
  if (value === null) {
    throw new Error('useToast must be used within ToastProvider.');
  }
  return value;
}
