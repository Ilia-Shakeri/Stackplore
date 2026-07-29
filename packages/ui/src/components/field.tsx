import {
  forwardRef,
  useId,
  type HTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
} from 'react';

import { FieldContext, useFieldContext } from './field-context';
import { cn } from '../lib/cn';

export interface FieldProps extends Omit<HTMLAttributes<HTMLDivElement>, 'children'> {
  children: ReactNode;
  controlId?: string;
  description?: ReactNode;
  disabled?: boolean;
  error?: ReactNode;
  label: ReactNode;
  required?: boolean;
}

export function Field({
  children,
  className,
  controlId,
  description,
  disabled = false,
  error,
  label,
  required = false,
  ...props
}: FieldProps) {
  const generatedId = useId();
  const inputId = controlId ?? `${generatedId}-control`;
  const descriptionId = description === undefined ? undefined : `${generatedId}-description`;
  const errorId = error === undefined ? undefined : `${generatedId}-error`;
  const describedBy = [descriptionId, errorId].filter(Boolean).join(' ') || undefined;
  const context = {
    controlId: inputId,
    describedBy,
    disabled,
    invalid: error !== undefined,
    required,
  };

  return (
    <FieldContext value={context}>
      <div
        className={cn('ui-field', className)}
        data-disabled={disabled || undefined}
        data-invalid={error === undefined ? undefined : true}
        {...props}
      >
        <label className="ui-field__label" htmlFor={inputId}>
          {label}
          {required ? (
            <span className="ui-field__required" aria-hidden="true">
              *
            </span>
          ) : null}
        </label>
        {children}
        {description === undefined ? null : (
          <div className="ui-field__description" id={descriptionId}>
            {description}
          </div>
        )}
        {error === undefined ? null : (
          <div className="ui-field__error" id={errorId} role="alert">
            {error}
          </div>
        )}
      </div>
    </FieldContext>
  );
}

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { 'aria-describedby': ariaDescribedBy, className, disabled, id, invalid, required, ...props },
  ref,
) {
  const field = useFieldContext();
  const describedBy = [ariaDescribedBy, field?.describedBy].filter(Boolean).join(' ') || undefined;
  const isDisabled = disabled ?? field?.disabled ?? false;
  const isInvalid = invalid ?? field?.invalid ?? false;
  const isRequired = required ?? field?.required ?? false;

  return (
    <input
      ref={ref}
      id={id ?? field?.controlId}
      className={cn('ui-input', className)}
      aria-describedby={describedBy}
      aria-invalid={isInvalid || undefined}
      disabled={isDisabled}
      required={isRequired}
      {...props}
    />
  );
});
