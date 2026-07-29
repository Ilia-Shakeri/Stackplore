import * as DropdownPrimitive from '@radix-ui/react-dropdown-menu';
import { Check, ChevronRight, Circle } from 'lucide-react';
import { forwardRef, type ComponentPropsWithoutRef, type ComponentRef } from 'react';

import { cn } from '../lib/cn';

export const Dropdown = DropdownPrimitive.Root;
export const DropdownTrigger = DropdownPrimitive.Trigger;
export const DropdownRadioGroup = DropdownPrimitive.RadioGroup;
export const DropdownSub = DropdownPrimitive.Sub;

export interface DropdownContentProps extends ComponentPropsWithoutRef<
  typeof DropdownPrimitive.Content
> {
  portalContainer?: HTMLElement | null;
}

export const DropdownContent = forwardRef<
  ComponentRef<typeof DropdownPrimitive.Content>,
  DropdownContentProps
>(function DropdownContent({ className, portalContainer, sideOffset = 6, ...props }, ref) {
  return (
    <DropdownPrimitive.Portal container={portalContainer}>
      <DropdownPrimitive.Content
        ref={ref}
        className={cn('ui-dropdown__content', className)}
        sideOffset={sideOffset}
        {...props}
      />
    </DropdownPrimitive.Portal>
  );
});

export interface DropdownItemProps extends ComponentPropsWithoutRef<typeof DropdownPrimitive.Item> {
  inset?: boolean;
  tone?: 'default' | 'danger';
}

export const DropdownItem = forwardRef<
  ComponentRef<typeof DropdownPrimitive.Item>,
  DropdownItemProps
>(function DropdownItem({ className, inset = false, tone = 'default', ...props }, ref) {
  return (
    <DropdownPrimitive.Item
      ref={ref}
      className={cn('ui-dropdown__item', className)}
      data-inset={inset || undefined}
      data-tone={tone}
      {...props}
    />
  );
});

export const DropdownCheckboxItem = forwardRef<
  ComponentRef<typeof DropdownPrimitive.CheckboxItem>,
  ComponentPropsWithoutRef<typeof DropdownPrimitive.CheckboxItem>
>(function DropdownCheckboxItem({ children, className, ...props }, ref) {
  return (
    <DropdownPrimitive.CheckboxItem
      ref={ref}
      className={cn('ui-dropdown__item', className)}
      {...props}
    >
      <span className="ui-dropdown__indicator">
        <DropdownPrimitive.ItemIndicator>
          <Check aria-hidden="true" />
        </DropdownPrimitive.ItemIndicator>
      </span>
      {children}
    </DropdownPrimitive.CheckboxItem>
  );
});

export const DropdownRadioItem = forwardRef<
  ComponentRef<typeof DropdownPrimitive.RadioItem>,
  ComponentPropsWithoutRef<typeof DropdownPrimitive.RadioItem>
>(function DropdownRadioItem({ children, className, ...props }, ref) {
  return (
    <DropdownPrimitive.RadioItem
      ref={ref}
      className={cn('ui-dropdown__item', className)}
      {...props}
    >
      <span className="ui-dropdown__indicator">
        <DropdownPrimitive.ItemIndicator>
          <Circle aria-hidden="true" />
        </DropdownPrimitive.ItemIndicator>
      </span>
      {children}
    </DropdownPrimitive.RadioItem>
  );
});

export const DropdownLabel = forwardRef<
  ComponentRef<typeof DropdownPrimitive.Label>,
  ComponentPropsWithoutRef<typeof DropdownPrimitive.Label> & { inset?: boolean }
>(function DropdownLabel({ className, inset = false, ...props }, ref) {
  return (
    <DropdownPrimitive.Label
      ref={ref}
      className={cn('ui-dropdown__label', className)}
      data-inset={inset || undefined}
      {...props}
    />
  );
});

export const DropdownSeparator = forwardRef<
  ComponentRef<typeof DropdownPrimitive.Separator>,
  ComponentPropsWithoutRef<typeof DropdownPrimitive.Separator>
>(function DropdownSeparator({ className, ...props }, ref) {
  return (
    <DropdownPrimitive.Separator
      ref={ref}
      className={cn('ui-dropdown__separator', className)}
      {...props}
    />
  );
});

export const DropdownSubTrigger = forwardRef<
  ComponentRef<typeof DropdownPrimitive.SubTrigger>,
  ComponentPropsWithoutRef<typeof DropdownPrimitive.SubTrigger> & { inset?: boolean }
>(function DropdownSubTrigger({ children, className, inset = false, ...props }, ref) {
  return (
    <DropdownPrimitive.SubTrigger
      ref={ref}
      className={cn('ui-dropdown__item', className)}
      data-inset={inset || undefined}
      {...props}
    >
      {children}
      <ChevronRight className="ui-dropdown__sub-icon" aria-hidden="true" />
    </DropdownPrimitive.SubTrigger>
  );
});

export const DropdownSubContent = forwardRef<
  ComponentRef<typeof DropdownPrimitive.SubContent>,
  ComponentPropsWithoutRef<typeof DropdownPrimitive.SubContent>
>(function DropdownSubContent({ className, sideOffset = 6, ...props }, ref) {
  return (
    <DropdownPrimitive.SubContent
      ref={ref}
      className={cn('ui-dropdown__content', className)}
      sideOffset={sideOffset}
      {...props}
    />
  );
});
