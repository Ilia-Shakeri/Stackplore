import { Component, type ErrorInfo, type ReactNode } from 'react';

export interface ErrorBoundaryFallbackProps {
  error: Error;
  resetErrorBoundary: () => void;
}

export interface ErrorBoundaryProps {
  children: ReactNode;
  fallback: ReactNode | ((props: ErrorBoundaryFallbackProps) => ReactNode);
  onError?: (error: Error, info: ErrorInfo) => void;
  onReset?: () => void;
  resetKeys?: readonly unknown[];
}

interface ErrorBoundaryState {
  error: Error | null;
}

function resetKeysChanged(
  previous: readonly unknown[] | undefined,
  next: readonly unknown[] | undefined,
) {
  if (previous === next) {
    return false;
  }
  if (previous === undefined || next === undefined || previous.length !== next.length) {
    return true;
  }
  return previous.some((value, index) => !Object.is(value, next[index]));
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  public override state: ErrorBoundaryState = { error: null };

  public static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    return {
      error: error instanceof Error ? error : new Error('A render error occurred.'),
    };
  }

  public override componentDidCatch(error: Error, info: ErrorInfo) {
    this.props.onError?.(error, info);
  }

  public override componentDidUpdate(previousProps: ErrorBoundaryProps) {
    if (
      this.state.error !== null &&
      resetKeysChanged(previousProps.resetKeys, this.props.resetKeys)
    ) {
      this.reset();
    }
  }

  private readonly reset = () => {
    this.props.onReset?.();
    this.setState({ error: null });
  };

  public override render() {
    const { error } = this.state;
    if (error === null) {
      return this.props.children;
    }

    return typeof this.props.fallback === 'function'
      ? this.props.fallback({ error, resetErrorBoundary: this.reset })
      : this.props.fallback;
  }
}
