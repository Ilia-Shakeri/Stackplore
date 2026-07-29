import { forwardRef, type HTMLAttributes } from 'react';

import { cn } from '../lib/cn';

export type BadgeTone = 'neutral' | 'info' | 'success' | 'warning' | 'danger';

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: BadgeTone;
}

export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(function Badge(
  { className, tone = 'neutral', ...props },
  ref,
) {
  return <span ref={ref} className={cn('ui-badge', className)} data-tone={tone} {...props} />;
});
