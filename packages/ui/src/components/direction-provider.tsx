import { DirectionProvider as PrimitiveDirectionProvider } from '@radix-ui/react-direction';
import type { ReactNode } from 'react';

export type InterfaceDirection = 'ltr' | 'rtl';

export interface DirectionProviderProps {
  children: ReactNode;
  direction: InterfaceDirection;
}

export function DirectionProvider({ children, direction }: DirectionProviderProps) {
  return <PrimitiveDirectionProvider dir={direction}>{children}</PrimitiveDirectionProvider>;
}
