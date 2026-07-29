import {
  forwardRef,
  type HTMLAttributes,
  type TableHTMLAttributes,
  type TdHTMLAttributes,
  type ThHTMLAttributes,
} from 'react';

import { cn } from '../lib/cn';

export interface TableFrameProps extends HTMLAttributes<HTMLDivElement> {
  'aria-label': string;
}

export const TableFrame = forwardRef<HTMLDivElement, TableFrameProps>(function TableFrame(
  { className, tabIndex = 0, ...props },
  ref,
) {
  return (
    <div ref={ref} className={cn('ui-table-frame', className)} tabIndex={tabIndex} {...props} />
  );
});

export const Table = forwardRef<HTMLTableElement, TableHTMLAttributes<HTMLTableElement>>(
  function Table({ className, ...props }, ref) {
    return <table ref={ref} className={cn('ui-table', className)} {...props} />;
  },
);

export const TableHeader = forwardRef<
  HTMLTableSectionElement,
  HTMLAttributes<HTMLTableSectionElement>
>(function TableHeader({ className, ...props }, ref) {
  return <thead ref={ref} className={cn('ui-table__header', className)} {...props} />;
});

export const TableBody = forwardRef<
  HTMLTableSectionElement,
  HTMLAttributes<HTMLTableSectionElement>
>(function TableBody({ className, ...props }, ref) {
  return <tbody ref={ref} className={cn('ui-table__body', className)} {...props} />;
});

export const TableFooter = forwardRef<
  HTMLTableSectionElement,
  HTMLAttributes<HTMLTableSectionElement>
>(function TableFooter({ className, ...props }, ref) {
  return <tfoot ref={ref} className={cn('ui-table__footer', className)} {...props} />;
});

export const TableRow = forwardRef<HTMLTableRowElement, HTMLAttributes<HTMLTableRowElement>>(
  function TableRow({ className, ...props }, ref) {
    return <tr ref={ref} className={cn('ui-table__row', className)} {...props} />;
  },
);

export const TableHead = forwardRef<HTMLTableCellElement, ThHTMLAttributes<HTMLTableCellElement>>(
  function TableHead({ className, scope = 'col', ...props }, ref) {
    return <th ref={ref} className={cn('ui-table__head', className)} scope={scope} {...props} />;
  },
);

export const TableCell = forwardRef<HTMLTableCellElement, TdHTMLAttributes<HTMLTableCellElement>>(
  function TableCell({ className, ...props }, ref) {
    return <td ref={ref} className={cn('ui-table__cell', className)} {...props} />;
  },
);

export const TableCaption = forwardRef<
  HTMLTableCaptionElement,
  HTMLAttributes<HTMLTableCaptionElement>
>(function TableCaption({ className, ...props }, ref) {
  return <caption ref={ref} className={cn('ui-table__caption', className)} {...props} />;
});
