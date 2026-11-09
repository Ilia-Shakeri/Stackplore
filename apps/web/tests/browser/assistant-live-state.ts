import type { Page } from '@playwright/test';

// Follow the existing explicit status control after its bounded stream ends.
// Never resubmit a question, change the stream lifetime or bypass result checks.
export async function waitForLiveAssistant(page: Page, followLabel: string): Promise<number> {
  const deadline = Date.now() + 110_000;
  let reconnections = 0;
  for (;;) {
    await page.waitForFunction(
      (follow) =>
        document.querySelectorAll('.assistant__turns .assistant__text').length > 0 ||
        Array.from(document.querySelectorAll('.assistant__actions button')).some(
          (button) =>
            button instanceof HTMLButtonElement &&
            !button.disabled &&
            button.getClientRects().length > 0 &&
            button.innerText.trim() === follow,
        ),
      followLabel,
      { timeout: Math.max(1, deadline - Date.now()) },
    );
    if ((await page.locator('.assistant__turns .assistant__text').count()) > 0)
      return reconnections;
    if (reconnections === 2 || Date.now() >= deadline)
      throw new Error('Bounded live status did not settle');
    await page
      .locator('.assistant')
      .getByRole('button', { name: followLabel, exact: true })
      .click({ timeout: Math.min(5000, Math.max(1, deadline - Date.now())) });
    reconnections++;
  }
}
