import * as SlotPrimitive from '@radix-ui/react-slot';
import { forwardRef, type ButtonHTMLAttributes, type MouseEvent, type ReactNode } from 'react';

import { cn } from '../lib/cn';

export type ButtonVariant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger';
export type ButtonSize = 'sm' | 'md' | 'lg' | 'icon';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean;
  loading?: boolean;
  size?: ButtonSize;
  variant?: ButtonVariant;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    asChild = false,
    'aria-busy': ariaBusy,
    'aria-disabled': ariaDisabled,
    children,
    className,
    disabled = false,
    loading = false,
    onClick,
    size = 'md',
    type,
    variant = 'primary',
    ...props
  },
  ref,
) {
  const Component = asChild ? SlotPrimitive.Slot : 'button';
  const unavailable = disabled || loading || ariaDisabled === true || ariaDisabled === 'true';

  const handleClick = (event: MouseEvent<HTMLButtonElement>) => {
    if (unavailable) {
      event.preventDefault();
      return;
    }
    onClick?.(event);
  };

  return (
    <Component
      ref={ref}
      className={cn('ui-button', className)}
      data-loading={loading || undefined}
      data-size={size}
      data-variant={variant}
      aria-busy={loading || ariaBusy || undefined}
      aria-disabled={unavailable || ariaDisabled || undefined}
      disabled={asChild ? undefined : unavailable}
      onClick={handleClick}
      type={asChild ? undefined : (type ?? 'button')}
      {...props}
    >
      {loading ? <span className="ui-button__spinner" aria-hidden="true" /> : null}
      {children}
    </Component>
  );
});

export interface IconButtonProps extends Omit<ButtonProps, 'aria-label' | 'size'> {
  'aria-label': string;
  children: ReactNode;
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  function IconButton(props, ref) {
    return <Button ref={ref} size="icon" {...props} />;
  },
);
