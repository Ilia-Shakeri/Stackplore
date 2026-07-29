import {
  Button,
  Field,
  Input,
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
import { render } from '@testing-library/react';
import axe from 'axe-core';
import { describe, expect, it } from 'vitest';

describe('representative interface accessibility', () => {
  it('has no detectable axe violations', async () => {
    const { container } = render(
      <main aria-labelledby="showcase-title">
        <h1 id="showcase-title">Interface showcase</h1>

        <form aria-label="Environment form">
          <Field label="Environment name" description="Use a name that operators know." required>
            <Input name="environment" />
          </Field>
          <Button type="submit">Save environment</Button>
        </form>

        <Tabs defaultValue="resources">
          <TabsList aria-label="Workspace views">
            <TabsTrigger value="resources">Resources</TabsTrigger>
            <TabsTrigger value="events">Events</TabsTrigger>
          </TabsList>
          <TabsContent value="resources">
            <TableFrame aria-label="Resource table">
              <Table>
                <TableCaption>Current resources</TableCaption>
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
            </TableFrame>
          </TabsContent>
          <TabsContent value="events">No events</TabsContent>
        </Tabs>
      </main>,
    );

    const results = await axe.run(container, {
      rules: {
        'color-contrast': { enabled: false },
      },
    });

    expect(results.violations).toEqual([]);
  });
});
