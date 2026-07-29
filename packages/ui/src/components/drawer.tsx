import * as DialogPrimitive from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import {
  forwardRef,
  type ComponentPropsWithoutRef,
  type ComponentRef,
  type ReactNode,
} from 'react';

import { cn } from '../lib/cn';

export const Drawer = DialogPrimitive.Root;
export const DrawerTrigger = DialogPrimitive.Trigger;
export const DrawerClose = DialogPrimitive.Close;

export type DrawerSide = 'start' | 'end' | 'bottom';
export type DrawerSize = 'sm' | 'md' | 'lg';

export interface DrawerContentProps extends Omit<
  ComponentPropsWithoutRef<typeof DialogPrimitive.Content>,
  'title'
> {
  children: ReactNode;
  closeLabel: string;
  description?: ReactNode;
  footer?: ReactNode;
  hideTitle?: boolean;
  side?: DrawerSide;
  size?: DrawerSize;
  title: ReactNode;
}

export const DrawerContent = forwardRef<
  ComponentRef<typeof DialogPrimitive.Content>,
  DrawerContentProps
>(function DrawerContent(
  {
    children,
    className,
    closeLabel,
    description,
    footer,
    hideTitle = false,
    side = 'start',
    size = 'md',
    title,
    ...props
  },
  ref,
) {
  const contentProps =
    description === undefined && props['aria-describedby'] === undefined
      ? { ...props, 'aria-describedby': undefined }
      : props;

  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="ui-overlay" />
      <DialogPrimitive.Content
        ref={ref}
        className={cn('ui-drawer', className)}
        data-side={side}
        data-size={size}
        {...contentProps}
      >
        <div className="ui-drawer__header">
          <DialogPrimitive.Title className={cn('ui-drawer__title', hideTitle && 'ui-sr-only')}>
            {title}
          </DialogPrimitive.Title>
          {description === undefined ? null : (
            <DialogPrimitive.Description className="ui-drawer__description">
              {description}
            </DialogPrimitive.Description>
          )}
        </div>
        <div className="ui-drawer__body">{children}</div>
        {footer === undefined ? null : <div className="ui-drawer__footer">{footer}</div>}
        <DialogPrimitive.Close className="ui-overlay-close ui-focus-ring" aria-label={closeLabel}>
          <X aria-hidden="true" />
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
});
