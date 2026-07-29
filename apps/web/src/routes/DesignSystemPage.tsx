import {
  Badge,
  Button,
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
  Dialog,
  DialogClose,
  DialogContent,
  DialogTrigger,
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerTrigger,
  Dropdown,
  DropdownCheckboxItem,
  DropdownContent,
  DropdownItem,
  DropdownLabel,
  DropdownRadioGroup,
  DropdownRadioItem,
  DropdownSeparator,
  DropdownSub,
  DropdownSubContent,
  DropdownSubTrigger,
  DropdownTrigger,
  ErrorBoundary,
  Field,
  IconButton,
  Input,
  Select,
  SelectGroup,
  SelectOption,
  Skeleton,
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFooter,
  TableFrame,
  TableHead,
  TableHeader,
  TableRow,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Tooltip,
  useToast,
} from '@stackplore/ui';
import { CircleHelp, Command } from '@stackplore/ui/icons';

export function DesignSystemPage() {
  const { notify } = useToast();

  const showToast = (tone: 'info' | 'success' = 'info') => {
    notify({
      title: tone === 'success' ? 'Action complete' : 'Design system notice',
      description: 'The toast queue is working.',
      tone,
    });
  };

  return (
    <div className="route-page design-showcase min-w-0">
      <header className="route-page__header">
        <p className="route-page__eyebrow">Development only</p>
        <h1>Design system</h1>
        <p>Project-owned controls, states, and tokens in one verification surface.</p>
      </header>

      <section className="showcase-section" aria-labelledby="showcase-actions">
        <div className="showcase-section__header">
          <h2 id="showcase-actions">Actions and status</h2>
          <p>Core button tones, compact actions, badges, and feedback.</p>
        </div>
        <Card tone="raised">
          <CardHeader>
            <CardTitle>Action controls</CardTitle>
            <CardDescription>Each control keeps visible focus and a clear name.</CardDescription>
          </CardHeader>
          <CardContent className="showcase-row">
            <Button
              onClick={() => {
                showToast('success');
              }}
            >
              Primary action
            </Button>
            <Button
              variant="secondary"
              onClick={() => {
                showToast();
              }}
            >
              Secondary action
            </Button>
            <Button variant="outline" disabled>
              Unavailable
            </Button>
            <Tooltip content="Keyboard shortcut help">
              <IconButton variant="ghost" aria-label="Show keyboard shortcut help">
                <CircleHelp aria-hidden="true" />
              </IconButton>
            </Tooltip>
          </CardContent>
          <CardFooter className="showcase-row">
            <Badge>Neutral</Badge>
            <Badge tone="info">Information</Badge>
            <Badge tone="success">Healthy</Badge>
            <Badge tone="warning">Warning</Badge>
            <Badge tone="danger">Error</Badge>
          </CardFooter>
        </Card>
      </section>

      <section className="showcase-section" aria-labelledby="showcase-fields">
        <div className="showcase-section__header">
          <h2 id="showcase-fields">Fields and menus</h2>
          <p>Labeled input, selection, menu, dialog, and drawer foundations.</p>
        </div>
        <div className="showcase-grid">
          <Card>
            <CardHeader>
              <CardTitle>Field controls</CardTitle>
            </CardHeader>
            <CardContent className="showcase-stack">
              <Field label="Workspace label" description="Local preview value only." required>
                <Input placeholder="Example workspace" />
              </Field>
              <div className="showcase-field">
                <label className="showcase-field__label" htmlFor="showcase-density">
                  Density
                </label>
                <Select id="showcase-density" defaultValue="comfortable">
                  <SelectGroup label="Layout density">
                    <SelectOption value="comfortable">Comfortable</SelectOption>
                    <SelectOption value="compact">Compact</SelectOption>
                  </SelectGroup>
                  <SelectGroup label="Automatic">
                    <SelectOption value="system">Use system choice</SelectOption>
                  </SelectGroup>
                </Select>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Layered controls</CardTitle>
            </CardHeader>
            <CardContent className="showcase-row">
              <Dropdown>
                <DropdownTrigger asChild>
                  <Button variant="secondary">
                    <Command aria-hidden="true" />
                    Open menu
                  </Button>
                </DropdownTrigger>
                <DropdownContent align="start">
                  <DropdownLabel>View options</DropdownLabel>
                  <DropdownCheckboxItem defaultChecked>Show labels</DropdownCheckboxItem>
                  <DropdownSeparator />
                  <DropdownRadioGroup defaultValue="comfortable">
                    <DropdownRadioItem value="comfortable">Comfortable</DropdownRadioItem>
                    <DropdownRadioItem value="compact">Compact</DropdownRadioItem>
                  </DropdownRadioGroup>
                  <DropdownSeparator />
                  <DropdownSub>
                    <DropdownSubTrigger>More options</DropdownSubTrigger>
                    <DropdownSubContent>
                      <DropdownItem
                        onSelect={() => {
                          showToast();
                        }}
                      >
                        Send notice
                      </DropdownItem>
                    </DropdownSubContent>
                  </DropdownSub>
                </DropdownContent>
              </Dropdown>

              <Dialog>
                <DialogTrigger asChild>
                  <Button variant="outline">Open dialog</Button>
                </DialogTrigger>
                <DialogContent
                  title="Dialog preview"
                  description="Focus remains inside this surface until it closes."
                  closeLabel="Close dialog"
                  footer={
                    <DialogClose asChild>
                      <Button>Done</Button>
                    </DialogClose>
                  }
                >
                  <p className="showcase-copy">Dialog body content stays short and direct.</p>
                </DialogContent>
              </Dialog>

              <Drawer>
                <DrawerTrigger asChild>
                  <Button variant="outline">Open drawer</Button>
                </DrawerTrigger>
                <DrawerContent
                  title="Drawer preview"
                  description="The same primitive powers mobile navigation."
                  closeLabel="Close drawer"
                  side="end"
                  footer={
                    <DrawerClose asChild>
                      <Button>Done</Button>
                    </DrawerClose>
                  }
                >
                  <p className="showcase-copy">Drawer body content uses a bounded side panel.</p>
                </DrawerContent>
              </Drawer>
            </CardContent>
          </Card>
        </div>
      </section>

      <section className="showcase-section" aria-labelledby="showcase-data">
        <div className="showcase-section__header">
          <h2 id="showcase-data">Structured content</h2>
          <p>Tabs, tables, loading placeholders, and the error boundary.</p>
        </div>
        <Tabs defaultValue="table">
          <TabsList aria-label="Structured content examples">
            <TabsTrigger value="table">Table</TabsTrigger>
            <TabsTrigger value="loading">Loading</TabsTrigger>
          </TabsList>
          <TabsContent value="table">
            <TableFrame aria-label="Design token table">
              <Table>
                <TableCaption>Example semantic design tokens</TableCaption>
                <TableHeader>
                  <TableRow>
                    <TableHead>Token</TableHead>
                    <TableHead>Role</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell>Surface</TableCell>
                    <TableCell>Primary content layer</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>Border</TableCell>
                    <TableCell>Visible separation</TableCell>
                  </TableRow>
                </TableBody>
                <TableFooter>
                  <TableRow>
                    <TableCell colSpan={2}>Two sample tokens</TableCell>
                  </TableRow>
                </TableFooter>
              </Table>
            </TableFrame>
          </TabsContent>
          <TabsContent value="loading">
            <div className="showcase-stack" role="status" aria-label="Loading preview">
              <Skeleton variant="text" width="45%" />
              <Skeleton variant="block" height="6rem" />
              <Skeleton variant="circle" width="2.5rem" height="2.5rem" />
            </div>
          </TabsContent>
        </Tabs>
        <ErrorBoundary fallback={<p role="alert">Preview could not render.</p>}>
          <p className="showcase-boundary-state">Error boundary ready.</p>
        </ErrorBoundary>
      </section>
    </div>
  );
}
