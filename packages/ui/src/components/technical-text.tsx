import type { ComponentPropsWithoutRef } from 'react';

import { cn } from '../lib/cn';

export type TechnicalTextProps = Omit<ComponentPropsWithoutRef<'bdi'>, 'dir'>;

export function TechnicalText({ className, ...props }: TechnicalTextProps) {
  return <bdi className={cn('ui-technical', className)} data-ui-technical dir="ltr" {...props} />;
}
