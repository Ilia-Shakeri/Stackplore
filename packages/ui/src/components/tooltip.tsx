import * as TooltipPrimitive from '@radix-ui/react-tooltip';
import { type ComponentPropsWithoutRef, type ReactElement, type ReactNode } from 'react';

import { cn } from '../lib/cn';

export interface TooltipProviderProps extends ComponentPropsWithoutRef<
  typeof TooltipPrimitive.Provider
> {
  children: ReactNode;
}

export function TooltipProvider({
  children,
  delayDuration = 400,
  skipDelayDuration = 200,
  ...props
}: TooltipProviderProps) {
  return (
    <TooltipPrimitive.Provider
      delayDuration={delayDuration}
      skipDelayDuration={skipDelayDuration}
      {...props}
    >
      {children}
    </TooltipPrimitive.Provider>
  );
}

export interface TooltipProps {
  align?: ComponentPropsWithoutRef<typeof TooltipPrimitive.Content>['align'];
  children: ReactElement;
  className?: string;
  content: ReactNode;
  delayDuration?: number;
  side?: ComponentPropsWithoutRef<typeof TooltipPrimitive.Content>['side'];
  sideOffset?: number;
}

export function Tooltip({
  align = 'center',
  children,
  className,
  content,
  delayDuration,
  side = 'top',
  sideOffset = 6,
}: TooltipProps) {
  const rootProps = delayDuration === undefined ? {} : { delayDuration };

  return (
    <TooltipPrimitive.Root {...rootProps}>
      <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
      <TooltipPrimitive.Portal>
        <TooltipPrimitive.Content
          className={cn('ui-tooltip', className)}
          align={align}
          side={side}
          sideOffset={sideOffset}
        >
          {content}
          <TooltipPrimitive.Arrow className="ui-tooltip__arrow" />
        </TooltipPrimitive.Content>
      </TooltipPrimitive.Portal>
    </TooltipPrimitive.Root>
  );
}
