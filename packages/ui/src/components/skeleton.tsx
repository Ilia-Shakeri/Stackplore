import { forwardRef, type CSSProperties, type HTMLAttributes } from 'react';

import { cn } from '../lib/cn';

export type SkeletonVariant = 'text' | 'block' | 'circle';

export interface SkeletonProps extends HTMLAttributes<HTMLDivElement> {
  height?: CSSProperties['height'];
  variant?: SkeletonVariant;
  width?: CSSProperties['width'];
}

export const Skeleton = forwardRef<HTMLDivElement, SkeletonProps>(function Skeleton(
  { className, height, style, variant = 'block', width, ...props },
  ref,
) {
  return (
    <div
      ref={ref}
      className={cn('ui-skeleton', className)}
      data-variant={variant}
      style={{ ...style, width, height }}
      {...props}
      aria-hidden="true"
    />
  );
});
