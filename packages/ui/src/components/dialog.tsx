import * as DialogPrimitive from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import {
  forwardRef,
  type ComponentPropsWithoutRef,
  type ComponentRef,
  type ReactNode,
} from 'react';

import { cn } from '../lib/cn';

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

export type DialogSize = 'sm' | 'md' | 'lg';

export interface DialogContentProps extends Omit<
  ComponentPropsWithoutRef<typeof DialogPrimitive.Content>,
  'title'
> {
  children: ReactNode;
  closeLabel: string;
  description?: ReactNode;
  footer?: ReactNode;
  hideTitle?: boolean;
  size?: DialogSize;
  title: ReactNode;
}

export const DialogContent = forwardRef<
  ComponentRef<typeof DialogPrimitive.Content>,
  DialogContentProps
>(function DialogContent(
  {
    children,
    className,
    closeLabel,
    description,
    footer,
    hideTitle = false,
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
        className={cn('ui-dialog', className)}
        data-size={size}
        {...contentProps}
      >
        <div className="ui-dialog__header">
          <DialogPrimitive.Title className={cn('ui-dialog__title', hideTitle && 'ui-sr-only')}>
            {title}
          </DialogPrimitive.Title>
          {description === undefined ? null : (
            <DialogPrimitive.Description className="ui-dialog__description">
              {description}
            </DialogPrimitive.Description>
          )}
        </div>
        <div className="ui-dialog__body">{children}</div>
        {footer === undefined ? null : <div className="ui-dialog__footer">{footer}</div>}
        <DialogPrimitive.Close className="ui-overlay-close ui-focus-ring" aria-label={closeLabel}>
          <X aria-hidden="true" />
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
});
