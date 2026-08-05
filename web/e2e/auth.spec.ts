import { test, expect } from '@playwright/test';

const TOKEN_KEY = 'seven_spade_auth_token';

test.describe('Authentication', () => {
  test('register', async ({ page, context }) => {
    await context.clearCookies();
    await page.goto('/register');
    await expect(page.getByRole('heading', { name: 'Create Account' })).toBeVisible();

    const ts = Date.now();
    await page.getByRole('textbox', { name: 'DISPLAY NAME' }).fill('E2eUser');
    await page.getByRole('textbox', { name: 'USERNAME' }).fill(`e2e${ts % 100000}`);
    await page.getByRole('textbox', { name: 'EMAIL' }).fill(`e2e${ts}@test.example`);
    const pwFields = page.getByRole('textbox', { name: /PASSWORD/i });
    await pwFields.nth(1).fill('password123');
    await pwFields.nth(0).fill('password123');
    await page.getByRole('checkbox').check();
    await page.getByRole('button', { name: 'Create Account' }).click();

    await page.waitForFunction(() => window.location.pathname === '/lobby', null, { timeout: 15000 });
    const token = await page.evaluate((k) => sessionStorage.getItem(k), TOKEN_KEY);
    expect(token).toBeTruthy();
  });

  test('login', async ({ page, context }) => {
    await context.clearCookies();
    const ts = Date.now();
    const email = `e2elog${ts}@test.example`;

    // register fresh
    await page.goto('/register');
    await expect(page.getByRole('heading', { name: 'Create Account' })).toBeVisible();
    await page.getByRole('textbox', { name: 'DISPLAY NAME' }).fill('LoginUser');
    await page.getByRole('textbox', { name: 'USERNAME' }).fill(`lg${ts % 100000}`);
    await page.getByRole('textbox', { name: 'EMAIL' }).fill(email);
    const pw = page.getByRole('textbox', { name: /PASSWORD/i });
    await pw.nth(1).fill('password123');
    await pw.nth(0).fill('password123');
    await page.getByRole('checkbox').check();
    await page.getByRole('button', { name: 'Create Account' }).click();
    await page.waitForFunction(() => window.location.pathname === '/lobby', null, { timeout: 15000 });

    // sign out
    await page.evaluate((k) => sessionStorage.removeItem(k), TOKEN_KEY);
    await context.clearCookies();
    await page.goto('/auth');
    await page.waitForLoadState('networkidle');

    await expect(page.getByRole('heading', { name: 'Take Your Seat' })).toBeVisible();
    await page.getByRole('tab', { name: 'Sign In' }).click();
    await page.getByLabel('Email').fill(email);
    await page.getByLabel('Password').fill('password123');
    await page.getByRole('button', { name: 'Sign In' }).last().click();

    await page.waitForFunction(() => window.location.pathname === '/lobby', null, { timeout: 15000 });
    const token = await page.evaluate((k) => sessionStorage.getItem(k), TOKEN_KEY);
    expect(token).toBeTruthy();
  });

  test('invalid login shows error', async ({ page, context }) => {
    await context.clearCookies();
    await page.goto('/auth');
    await expect(page.getByRole('heading', { name: 'Take Your Seat' })).toBeVisible();
    await page.getByRole('tab', { name: 'Sign In' }).click();
    await page.getByLabel('Email').fill('noone@test.example');
    await page.getByLabel('Password').fill('wrong');
    await page.getByRole('button', { name: 'Sign In' }).last().click();

    await expect(page.getByText('Invalid email or password')).toBeVisible({ timeout: 8000 });
  });

  test('password mismatch', async ({ page, context }) => {
    await context.clearCookies();
    await page.goto('/register');
    await expect(page.getByRole('heading', { name: 'Create Account' })).toBeVisible();
    await page.getByRole('textbox', { name: 'DISPLAY NAME' }).fill('Test');
    await page.getByRole('textbox', { name: 'USERNAME' }).fill('testx99');
    await page.getByRole('textbox', { name: 'EMAIL' }).fill('t@t.com');
    const pw = page.getByRole('textbox', { name: /PASSWORD/i });
    await pw.nth(1).fill('different1');
    await pw.nth(0).fill('password1');
    await page.getByRole('checkbox').check();
    await page.getByRole('button', { name: 'Create Account' }).click();

    await expect(page.getByText('Passwords do not match')).toBeVisible({ timeout: 5000 });
  });
});
