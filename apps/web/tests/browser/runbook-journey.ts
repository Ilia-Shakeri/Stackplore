import { expect, type Locator, type Page } from '@playwright/test';

type Labels = (key: string) => string;
type PrivateInput = (locator: Locator, value: string) => Promise<void>;
const prose = (locale: 'en' | 'fa') =>
  locale === 'en'
    ? {
        title: 'Reviewed browser service',
        body: 'Inspect service health. <script>inert browser prose</script>',
        query: 'service',
      }
    : {
        title: 'راهنمای سرویس مرورگر',
        body: 'سلامت سرویس را بررسی کنید. <script>متن ساده</script>',
        query: 'سرویس',
      };
const corpus = (version: number) => ({
  id: 'browser_approved',
  version,
  audience: 'all_users',
  provenance: {
    kind: 'operator_authored',
    source: 'Synthetic browser fixture',
    license: 'Fixture author permission',
    use_reviewed: true,
    assistance_reviewed: true,
  },
  documents: (['en', 'fa'] as const).flatMap((locale) => [
    {
      id: 'recovery',
      locale,
      ...{ title: prose(locale).title, body: prose(locale).body },
    },
    {
      id: 'safe_check',
      locale,
      title: locale === 'en' ? 'Local check guide' : 'راهنمای بررسی محلی',
      body:
        locale === 'en'
          ? 'Inspect current service health.'
          : 'سلامت سرویس را بررسی کنید.',
    },
  ]),
});
async function search(page: Page, locale: 'en' | 'fa', label: Labels) {
  await page.getByRole('button', { name: label('runbooks.browse'), exact: true }).click();
  await page
    .getByRole('textbox', { name: label('runbooks.query'), exact: true })
    .fill(prose(locale).query);
  await page.locator('.runbooks-search button[type="submit"]').click();
}
async function openResult(page: Page, locale: 'en' | 'fa', label: Labels) {
  await search(page, locale, label);
  await page.getByRole('button', { name: prose(locale).title, exact: true }).click();
  await expect(page.locator('.runbooks-document')).toContainText(prose(locale).body);
  await expect(page.locator('.runbooks-document script')).toHaveCount(0);
  await expect(page.locator('.runbooks-document')).toContainText('Synthetic browser fixture');
  await expect(page.locator('.runbooks-document')).toContainText('Fixture author permission');
}
export async function runbookAdminJourney(
  page: Page,
  locale: 'en' | 'fa',
  password: string,
  label: Labels,
  privateInput: PrivateInput,
) {
  await page.goto('/assistant/runbooks');
  await search(page, locale, label);
  await expect(page.getByText(label('runbooks.empty'), { exact: true })).toBeVisible();
  const manage = async () => {
    await page.getByRole('button', { name: label('runbooks.manage'), exact: true }).click();
    await page.getByRole('button', { name: label('runbooks.refresh'), exact: true }).click();
    await expect(
      page.getByRole('textbox', { name: label('runbooks.json'), exact: true }),
    ).toBeVisible();
  };
  const commit = async () => {
    await page.getByRole('checkbox', { name: label('runbooks.confirm'), exact: true }).check();
    await privateInput(page.locator('input[name="current_password"]'), password);
    await page.getByRole('button', { name: label('runbooks.commit'), exact: true }).click();
    await expect(page.getByText(label('runbooks.saved'), { exact: true })).toBeVisible();
    await expect(page.locator('input[type="password"]')).toHaveCount(0);
  };
  const approve = async (version: number) => {
    await manage();
    await page
      .getByRole('textbox', { name: label('runbooks.json'), exact: true })
      .fill(JSON.stringify(corpus(version)));
    await page.getByRole('button', { name: label('runbooks.preview'), exact: true }).click();
    await expect(
      page.getByRole('region', { name: label('runbooks.review'), exact: true }),
    ).toBeVisible();
    await commit();
  };
  await approve(1);
  await openResult(page, locale, label);
  await manage();
  await page.getByRole('button', { name: label('runbooks.revoke'), exact: true }).click();
  await expect(page.getByText(label('runbooks.revokeNotice'), { exact: true })).toBeVisible();
  await commit();
  await search(page, locale, label);
  await expect(page.getByText(label('runbooks.empty'), { exact: true })).toBeVisible();
  await approve(2);
  await openResult(page, locale, label);
}
export async function runbookViewerJourney(page: Page, locale: 'en' | 'fa', label: Labels) {
  await page.goto('/assistant/runbooks');
  await expect(
    page.getByRole('button', { name: label('runbooks.manage'), exact: true }),
  ).toHaveCount(0);
  expect((await page.request.get('/api/v1/assistant/runbooks')).status()).toBe(403);
  await openResult(page, locale, label);
}
