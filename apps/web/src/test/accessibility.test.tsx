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
import { createMemoryHistory } from '@tanstack/react-router';
import { render, screen } from '@testing-library/react';
import axe from 'axe-core';
import { describe, expect, it } from 'vitest';

import { App } from '../app/App';
import { createAppRouter } from '../app/router';
import { createAuthenticatedAuthStore, createResolvedAuthStore, createTestUser } from './auth';
import { createTestUsersApi } from './users';

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

  it('has no detectable violations in the Persian RTL shell', async () => {
    globalThis.localStorage.setItem('stackplore.locale', 'fa');
    const history = createMemoryHistory({ initialEntries: ['/overview'] });
    const router = createAppRouter(history, createAuthenticatedAuthStore());
    const { container } = render(<App router={router} />);

    expect(await screen.findByRole('heading', { level: 1, name: 'نمای کلی' })).not.toBeNull();

    const results = await axe.run(container, {
      rules: {
        'color-contrast': { enabled: false },
      },
    });

    expect(results.violations).toEqual([]);
  });

  it('has no detectable violations in the local login form', async () => {
    const history = createMemoryHistory({ initialEntries: ['/login'] });
    const router = createAppRouter(history, createResolvedAuthStore({ status: 'unauthenticated' }));
    const { container } = render(<App router={router} />);

    expect(await screen.findByRole('heading', { level: 1, name: 'Sign in' })).not.toBeNull();
    const results = await axe.run(container, {
      rules: {
        'color-contrast': { enabled: false },
      },
    });

    expect(results.violations).toEqual([]);
  });

  it('has no detectable violations in user administration', async () => {
    const history = createMemoryHistory({ initialEntries: ['/settings/users'] });
    const router = createAppRouter(
      history,
      createAuthenticatedAuthStore(),
      createTestUsersApi({
        listUsers: () => Promise.resolve({ items: [createTestUser()] }),
      }),
    );
    const { container } = render(<App router={router} />);

    expect(await screen.findByRole('heading', { level: 1, name: 'Users' })).not.toBeNull();
    expect(await screen.findByRole('table', { name: 'Current local users' })).not.toBeNull();
    const results = await axe.run(container, {
      rules: {
        'color-contrast': { enabled: false },
      },
    });

    expect(results.violations).toEqual([]);
  });
});
