import { forwardRef, type HTMLAttributes } from 'react';

import { cn } from '../lib/cn';

export type CardTone = 'default' | 'muted' | 'raised';

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  tone?: CardTone;
}

export const Card = forwardRef<HTMLDivElement, CardProps>(function Card(
  { className, tone = 'default', ...props },
  ref,
) {
  return <div ref={ref} className={cn('ui-card', className)} data-tone={tone} {...props} />;
});

export const CardHeader = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function CardHeader({ className, ...props }, ref) {
    return <div ref={ref} className={cn('ui-card__header', className)} {...props} />;
  },
);

export const CardTitle = forwardRef<HTMLHeadingElement, HTMLAttributes<HTMLHeadingElement>>(
  function CardTitle({ className, ...props }, ref) {
    return <h2 ref={ref} className={cn('ui-card__title', className)} {...props} />;
  },
);

export const CardDescription = forwardRef<
  HTMLParagraphElement,
  HTMLAttributes<HTMLParagraphElement>
>(function CardDescription({ className, ...props }, ref) {
  return <p ref={ref} className={cn('ui-card__description', className)} {...props} />;
});

export const CardContent = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function CardContent({ className, ...props }, ref) {
    return <div ref={ref} className={cn('ui-card__content', className)} {...props} />;
  },
);

export const CardFooter = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function CardFooter({ className, ...props }, ref) {
    return <div ref={ref} className={cn('ui-card__footer', className)} {...props} />;
  },
);
