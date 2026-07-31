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

import { useLocale } from '../i18n';

export function DesignSystemPage() {
  const { notify } = useToast();
  const { t, tPlural } = useLocale();

  const showToast = (tone: 'info' | 'success' = 'info') => {
    notify({
      title: tone === 'success' ? t('showcase.toast.actionComplete') : t('showcase.toast.notice'),
      description: t('showcase.toast.body'),
      tone,
    });
  };

  return (
    <div className="route-page design-showcase min-w-0">
      <header className="route-page__header">
        <p className="route-page__eyebrow">{t('showcase.eyebrow')}</p>
        <h1>{t('route.designSystem.title')}</h1>
        <p>{t('showcase.intro')}</p>
      </header>

      <section className="showcase-section" aria-labelledby="showcase-actions">
        <div className="showcase-section__header">
          <h2 id="showcase-actions">{t('showcase.actions.title')}</h2>
          <p>{t('showcase.actions.body')}</p>
        </div>
        <Card tone="raised">
          <CardHeader>
            <CardTitle>{t('showcase.actions.cardTitle')}</CardTitle>
            <CardDescription>{t('showcase.actions.cardBody')}</CardDescription>
          </CardHeader>
          <CardContent className="showcase-row">
            <Button
              onClick={() => {
                showToast('success');
              }}
            >
              {t('showcase.actions.primary')}
            </Button>
            <Button
              variant="secondary"
              onClick={() => {
                showToast();
              }}
            >
              {t('showcase.actions.secondary')}
            </Button>
            <Button variant="outline" disabled>
              {t('showcase.actions.unavailable')}
            </Button>
            <Tooltip content={t('showcase.actions.shortcutHelp')}>
              <IconButton variant="ghost" aria-label={t('showcase.actions.showShortcutHelp')}>
                <CircleHelp aria-hidden="true" />
              </IconButton>
            </Tooltip>
          </CardContent>
          <CardFooter className="showcase-row">
            <Badge>{t('showcase.status.neutral')}</Badge>
            <Badge tone="info">{t('showcase.status.information')}</Badge>
            <Badge tone="success">{t('showcase.status.healthy')}</Badge>
            <Badge tone="warning">{t('showcase.status.warning')}</Badge>
            <Badge tone="danger">{t('showcase.status.error')}</Badge>
          </CardFooter>
        </Card>
      </section>

      <section className="showcase-section" aria-labelledby="showcase-fields">
        <div className="showcase-section__header">
          <h2 id="showcase-fields">{t('showcase.fields.title')}</h2>
          <p>{t('showcase.fields.body')}</p>
        </div>
        <div className="showcase-grid">
          <Card>
            <CardHeader>
              <CardTitle>{t('showcase.fields.cardTitle')}</CardTitle>
            </CardHeader>
            <CardContent className="showcase-stack">
              <Field
                label={t('showcase.fields.workspaceLabel')}
                description={t('showcase.fields.workspaceHelp')}
                required
              >
                <Input placeholder={t('showcase.fields.workspacePlaceholder')} />
              </Field>
              <div className="showcase-field">
                <label className="showcase-field__label" htmlFor="showcase-density">
                  {t('showcase.fields.density')}
                </label>
                <Select id="showcase-density" defaultValue="comfortable">
                  <SelectGroup label={t('showcase.fields.densityGroup')}>
                    <SelectOption value="comfortable">
                      {t('showcase.fields.comfortable')}
                    </SelectOption>
                    <SelectOption value="compact">{t('showcase.fields.compact')}</SelectOption>
                  </SelectGroup>
                  <SelectGroup label={t('showcase.fields.automaticGroup')}>
                    <SelectOption value="system">{t('showcase.fields.systemChoice')}</SelectOption>
                  </SelectGroup>
                </Select>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t('showcase.layers.title')}</CardTitle>
            </CardHeader>
            <CardContent className="showcase-row">
              <Dropdown>
                <DropdownTrigger asChild>
                  <Button variant="secondary">
                    <Command aria-hidden="true" />
                    {t('showcase.layers.openMenu')}
                  </Button>
                </DropdownTrigger>
                <DropdownContent align="start">
                  <DropdownLabel>{t('showcase.layers.viewOptions')}</DropdownLabel>
                  <DropdownCheckboxItem defaultChecked>
                    {t('showcase.layers.showLabels')}
                  </DropdownCheckboxItem>
                  <DropdownSeparator />
                  <DropdownRadioGroup defaultValue="comfortable">
                    <DropdownRadioItem value="comfortable">
                      {t('showcase.fields.comfortable')}
                    </DropdownRadioItem>
                    <DropdownRadioItem value="compact">
                      {t('showcase.fields.compact')}
                    </DropdownRadioItem>
                  </DropdownRadioGroup>
                  <DropdownSeparator />
                  <DropdownSub>
                    <DropdownSubTrigger>{t('showcase.layers.moreOptions')}</DropdownSubTrigger>
                    <DropdownSubContent>
                      <DropdownItem
                        onSelect={() => {
                          showToast();
                        }}
                      >
                        {t('showcase.layers.sendNotice')}
                      </DropdownItem>
                    </DropdownSubContent>
                  </DropdownSub>
                </DropdownContent>
              </Dropdown>

              <Dialog>
                <DialogTrigger asChild>
                  <Button variant="outline">{t('showcase.layers.dialogOpen')}</Button>
                </DialogTrigger>
                <DialogContent
                  title={t('showcase.layers.dialogTitle')}
                  description={t('showcase.layers.dialogDescription')}
                  closeLabel={t('showcase.layers.dialogClose')}
                  footer={
                    <DialogClose asChild>
                      <Button>{t('showcase.layers.dialogDone')}</Button>
                    </DialogClose>
                  }
                >
                  <p className="showcase-copy">{t('showcase.layers.dialogBody')}</p>
                </DialogContent>
              </Dialog>

              <Drawer>
                <DrawerTrigger asChild>
                  <Button variant="outline">{t('showcase.layers.drawerOpen')}</Button>
                </DrawerTrigger>
                <DrawerContent
                  title={t('showcase.layers.drawerTitle')}
                  description={t('showcase.layers.drawerDescription')}
                  closeLabel={t('showcase.layers.drawerClose')}
                  side="end"
                  footer={
                    <DrawerClose asChild>
                      <Button>{t('showcase.layers.drawerDone')}</Button>
                    </DrawerClose>
                  }
                >
                  <p className="showcase-copy">{t('showcase.layers.drawerBody')}</p>
                </DrawerContent>
              </Drawer>
            </CardContent>
          </Card>
        </div>
      </section>

      <section className="showcase-section" aria-labelledby="showcase-data">
        <div className="showcase-section__header">
          <h2 id="showcase-data">{t('showcase.structured.title')}</h2>
          <p>{t('showcase.structured.body')}</p>
        </div>
        <Tabs defaultValue="table">
          <TabsList aria-label={t('showcase.structured.tabsLabel')}>
            <TabsTrigger value="table">{t('showcase.structured.tableTab')}</TabsTrigger>
            <TabsTrigger value="loading">{t('showcase.structured.loadingTab')}</TabsTrigger>
          </TabsList>
          <TabsContent value="table">
            <TableFrame aria-label={t('showcase.structured.tableLabel')}>
              <Table>
                <TableCaption>{t('showcase.structured.tableCaption')}</TableCaption>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('showcase.structured.token')}</TableHead>
                    <TableHead>{t('showcase.structured.role')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  <TableRow>
                    <TableCell>{t('showcase.structured.surface')}</TableCell>
                    <TableCell>{t('showcase.structured.surfaceRole')}</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell>{t('showcase.structured.border')}</TableCell>
                    <TableCell>{t('showcase.structured.borderRole')}</TableCell>
                  </TableRow>
                </TableBody>
                <TableFooter>
                  <TableRow>
                    <TableCell colSpan={2}>
                      {tPlural('showcase.structured.tokenCount', 2)}
                    </TableCell>
                  </TableRow>
                </TableFooter>
              </Table>
            </TableFrame>
          </TabsContent>
          <TabsContent value="loading">
            <div
              className="showcase-stack"
              role="status"
              aria-label={t('showcase.structured.loadingLabel')}
            >
              <Skeleton variant="text" width="45%" />
              <Skeleton variant="block" height="6rem" />
              <Skeleton variant="circle" width="2.5rem" height="2.5rem" />
            </div>
          </TabsContent>
        </Tabs>
        <ErrorBoundary fallback={<p role="alert">{t('showcase.structured.renderError')}</p>}>
          <p className="showcase-boundary-state">{t('showcase.structured.boundaryReady')}</p>
        </ErrorBoundary>
      </section>
    </div>
  );
}
