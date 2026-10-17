import { fireEvent, render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { AuthProvider } from './auth-context';
import { createAuthenticatedAuthStore } from '../../test/auth';

describe('session activity boundary', () => {
  it('records navigation and scrolling activity only while the provider is mounted', () => {
    const store = createAuthenticatedAuthStore();
    const activity = vi.spyOn(store, 'recordActivity').mockResolvedValue(undefined);
    const { unmount } = render(
      <AuthProvider store={store}>
        <p>Resource</p>
      </AuthProvider>,
    );
    fireEvent.pointerDown(window);
    fireEvent.keyDown(window, { key: 'ArrowDown' });
    fireEvent.wheel(window);
    fireEvent.touchMove(window);
    expect(activity).toHaveBeenCalledTimes(4);
    unmount();
    fireEvent.pointerDown(window);
    fireEvent.keyDown(window, { key: 'ArrowDown' });
    fireEvent.wheel(window);
    fireEvent.touchMove(window);
    expect(activity).toHaveBeenCalledTimes(4);
  });
});
