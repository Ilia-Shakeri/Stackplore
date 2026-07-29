import {
  Button,
  Dialog,
  DialogContent,
  Field,
  Input,
  Select,
  SelectOption,
  Skeleton,
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFrame,
  TableHead,
  TableHeader,
  TableRow,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@stackplore/ui';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';

describe('form primitives', () => {
  it('keeps a loading button unavailable', () => {
    const handleClick = vi.fn();

    render(
      <Button loading onClick={handleClick}>
        Save changes
      </Button>,
    );

    const button = screen.getByRole('button', { name: 'Save changes' });

    expect(button.getAttribute('type')).toBe('button');
    expect(button.getAttribute('aria-busy')).toBe('true');
    expect(button.getAttribute('aria-disabled')).toBe('true');
    expect((button as HTMLButtonElement).disabled).toBe(true);

    fireEvent.click(button);

    expect(handleClick).not.toHaveBeenCalled();
  });

  it('joins a field label, help text, error, and input state', () => {
    render(
      <Field
        label="Environment name"
        description="Use a name that operators know."
        error="Name is required."
        required
      >
        <Input name="environment" />
      </Field>,
    );

    const input = screen.getByRole('textbox', { name: 'Environment name' });
    const description = screen.getByText('Use a name that operators know.');
    const error = screen.getByText('Name is required.');
    const describedBy = input.getAttribute('aria-describedby')?.split(' ') ?? [];

    expect(input.id).not.toBe('');
    expect(input.getAttribute('aria-invalid')).toBe('true');
    expect((input as HTMLInputElement).required).toBe(true);
    expect(describedBy).toContain(description.id);
    expect(describedBy).toContain(error.id);
  });

  it('joins field state to a native select', () => {
    render(
      <Field
        label="Environment"
        description="Choose a source."
        error="A source is required."
        disabled
        required
      >
        <Select defaultValue="">
          <SelectOption value="">Choose one</SelectOption>
          <SelectOption value="demo">Demo</SelectOption>
        </Select>
      </Field>,
    );

    const select = screen.getByRole('combobox', { name: 'Environment' });
    const description = screen.getByText('Choose a source.');
    const error = screen.getByText('A source is required.');
    const describedBy = select.getAttribute('aria-describedby')?.split(' ') ?? [];

    expect(select.id).not.toBe('');
    expect(select.getAttribute('aria-invalid')).toBe('true');
    expect((select as HTMLSelectElement).disabled).toBe(true);
    expect((select as HTMLSelectElement).required).toBe(true);
    expect(describedBy).toContain(description.id);
    expect(describedBy).toContain(error.id);
  });

  it('blocks a caller-declared unavailable button', () => {
    const handleClick = vi.fn();

    render(
      <Button aria-disabled onClick={handleClick}>
        Guarded action
      </Button>,
    );

    const button = screen.getByRole('button', { name: 'Guarded action' });

    expect(button.getAttribute('aria-disabled')).toBe('true');
    expect((button as HTMLButtonElement).disabled).toBe(true);

    fireEvent.click(button);

    expect(handleClick).not.toHaveBeenCalled();
  });
});

function DialogHarness() {
  const [open, setOpen] = useState(true);

  return (
    <>
      <output data-testid="dialog-state">{open ? 'open' : 'closed'}</output>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent
          closeLabel="Close details"
          description="Review the selected resource."
          title="Resource details"
        >
          <p>Resource body</p>
        </DialogContent>
      </Dialog>
    </>
  );
}

describe('overlay primitive', () => {
  it('names a dialog and closes through its safe close control', () => {
    render(<DialogHarness />);

    const dialog = screen.getByRole('dialog', { name: 'Resource details' });
    const descriptionId = dialog.getAttribute('aria-describedby');

    expect(descriptionId).not.toBeNull();
    expect(document.getElementById(descriptionId ?? '')?.textContent).toBe(
      'Review the selected resource.',
    );

    fireEvent.click(screen.getByRole('button', { name: 'Close details' }));

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.getByTestId('dialog-state').textContent).toBe('closed');
  });
});

describe('data display primitives', () => {
  it('renders tabs with selected state and linked content', () => {
    render(
      <Tabs defaultValue="overview">
        <TabsList aria-label="Resource views">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="health">Health</TabsTrigger>
        </TabsList>
        <TabsContent value="overview">Overview content</TabsContent>
        <TabsContent value="health">Health content</TabsContent>
      </Tabs>,
    );

    const overviewTab = screen.getByRole('tab', { name: 'Overview' });
    const healthTab = screen.getByRole('tab', { name: 'Health' });
    const panel = screen.getByRole('tabpanel');

    expect(overviewTab.getAttribute('aria-selected')).toBe('true');
    expect(healthTab.getAttribute('aria-selected')).toBe('false');
    expect(overviewTab.getAttribute('aria-controls')).toBe(panel.id);
    expect(panel.getAttribute('aria-labelledby')).toBe(overviewTab.id);
    expect(panel.textContent).toBe('Overview content');
  });

  it('keeps a table semantic and its scroll frame named', () => {
    render(
      <TableFrame aria-label="Resource table">
        <Table>
          <TableCaption>Resource health</TableCaption>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Status</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow>
              <TableCell>Gateway</TableCell>
              <TableCell>Healthy</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </TableFrame>,
    );

    const frame = screen.getByLabelText('Resource table');
    const table = within(frame).getByRole('table', { name: 'Resource health' });
    const headers = within(table).getAllByRole('columnheader');

    expect(frame.getAttribute('tabindex')).toBe('0');
    expect(headers).toHaveLength(2);
    expect(headers[0]?.getAttribute('scope')).toBe('col');
    expect(within(table).getByText('Gateway').tagName).toBe('TD');
  });

  it('hides a sized skeleton from the accessibility tree', () => {
    const { container } = render(<Skeleton variant="circle" width={32} height={32} />);
    const skeleton = container.querySelector('.ui-skeleton');

    expect(skeleton).not.toBeNull();
    expect(skeleton?.getAttribute('aria-hidden')).toBe('true');
    expect(skeleton?.getAttribute('data-variant')).toBe('circle');
    expect((skeleton as HTMLElement | null)?.style.width).toBe('32px');
    expect((skeleton as HTMLElement | null)?.style.height).toBe('32px');
  });
});
