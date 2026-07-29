import { ChevronDown } from 'lucide-react';
import {
  forwardRef,
  type OptgroupHTMLAttributes,
  type OptionHTMLAttributes,
  type SelectHTMLAttributes,
} from 'react';

import { useFieldContext } from './field-context';
import { cn } from '../lib/cn';

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  invalid?: boolean;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select(
  {
    'aria-describedby': ariaDescribedBy,
    children,
    className,
    disabled,
    id,
    invalid,
    required,
    ...props
  },
  ref,
) {
  const field = useFieldContext();
  const describedBy = [ariaDescribedBy, field?.describedBy].filter(Boolean).join(' ') || undefined;
  const isDisabled = disabled ?? field?.disabled ?? false;
  const isInvalid = invalid ?? field?.invalid ?? false;
  const isRequired = required ?? field?.required ?? false;

  return (
    <span className="ui-select">
      <select
        ref={ref}
        id={id ?? field?.controlId}
        className={cn('ui-select__control', className)}
        aria-describedby={describedBy}
        aria-invalid={isInvalid || undefined}
        disabled={isDisabled}
        required={isRequired}
        {...props}
      >
        {children}
      </select>
      <ChevronDown className="ui-select__icon" aria-hidden="true" />
    </span>
  );
});

export const SelectOption = forwardRef<HTMLOptionElement, OptionHTMLAttributes<HTMLOptionElement>>(
  function SelectOption(props, ref) {
    return <option ref={ref} {...props} />;
  },
);

export interface SelectGroupProps extends OptgroupHTMLAttributes<HTMLOptGroupElement> {
  label: string;
}

export const SelectGroup = forwardRef<HTMLOptGroupElement, SelectGroupProps>(
  function SelectGroup(props, ref) {
    return <optgroup ref={ref} {...props} />;
  },
);
