import { createContext, useContext, type ReactNode } from 'react';
import type { AssistantApi } from './assistant-api';

const AssistantContext = createContext<AssistantApi | null>(null);
export function AssistantProvider({
  api,
  children,
}: {
  readonly api: AssistantApi;
  readonly children: ReactNode;
}) {
  return <AssistantContext value={api}>{children}</AssistantContext>;
}
export const useOptionalAssistantApi = () => useContext(AssistantContext);
