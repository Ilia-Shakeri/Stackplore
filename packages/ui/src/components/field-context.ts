import { createContext, useContext } from 'react';

export interface FieldContextValue {
  controlId: string;
  describedBy: string | undefined;
  disabled: boolean;
  invalid: boolean;
  required: boolean;
}

export const FieldContext = createContext<FieldContextValue | null>(null);

export function useFieldContext() {
  return useContext(FieldContext);
}
