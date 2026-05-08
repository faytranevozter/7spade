import { test, expect } from '@playwright/test';

test.describe('Authentication Flows', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate first, then clear storage
    await page.goto('http://localhost:5173/mock/auth');
    await page.context().clearCookies();
    await page.evaluate(() => {
      localStorage.clear();
    });
  });

  test('Register flow: should successfully register a new user', async ({ page }) => {
    // Start the backend (assumes docker-compose is running)
    await page.goto('http://localhost:5173/mock/register');
    
    // Wait for page to load
    await expect(page.getByRole('heading', { name: 'Register' })).toBeVisible();
    
    // Fill registration form
    const timestamp = Date.now();
    const email = `test${timestamp}@example.com`;
    await page.getByLabel('Email').fill(email);
    await page.getByLabel('Display name').fill('Test User');
    await page.getByLabel('Password', { exact: true }).fill('password123');
    await page.getByLabel('Confirm password').fill('password123');
    
    // Take screenshot before submission
    await page.screenshot({ path: 'e2e/screenshots/register-filled.png', fullPage: true });
    
    // Submit form
    await page.getByRole('button', { name: 'Create account' }).click();
    
    // Should navigate to lobby
    await expect(page).toHaveURL(/\/mock\/lobby/);
    
    // Verify JWT is stored in localStorage
    const token = await page.evaluate(() => localStorage.getItem('seven_spade_auth_token'));
    expect(token).toBeTruthy();
    expect(token).not.toContain('guest'); // Should be a user token, not guest
    
    // Take screenshot of success state
    await page.screenshot({ path: 'e2e/screenshots/register-success.png', fullPage: true });
  });

  test('Login flow: should successfully login with existing user', async ({ page }) => {
    // First register a user
    const timestamp = Date.now();
    const email = `login${timestamp}@example.com`;
    const password = 'password123';
    
    await page.goto('http://localhost:5173/mock/register');
    await page.getByLabel('Email').fill(email);
    await page.getByLabel('Display name').fill('Login Test User');
    await page.getByLabel('Password', { exact: true }).fill(password);
    await page.getByLabel('Confirm password').fill(password);
    await page.getByRole('button', { name: 'Create account' }).click();
    await expect(page).toHaveURL(/\/mock\/lobby/);
    
    // Logout (clear localStorage)
    await page.evaluate(() => localStorage.clear());
    
    // Now test login
    await page.goto('http://localhost:5173/mock/login');
    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
    
    // Fill login form
    await page.getByLabel('Email').fill(email);
    await page.getByLabel('Password').fill(password);
    
    // Take screenshot before submission
    await page.screenshot({ path: 'e2e/screenshots/login-filled.png', fullPage: true });
    
    // Submit form
    await page.getByRole('button', { name: 'Login' }).click();
    
    // Should navigate to lobby
    await expect(page).toHaveURL(/\/mock\/lobby/);
    
    // Verify JWT is stored
    const token = await page.evaluate(() => localStorage.getItem('seven_spade_auth_token'));
    expect(token).toBeTruthy();
    
    // Take screenshot of success state
    await page.screenshot({ path: 'e2e/screenshots/login-success.png', fullPage: true });
  });

  test('Register error: should show error for duplicate email', async ({ page }) => {
    // Register a user first
    const email = `duplicate@example.com`;
    await page.goto('http://localhost:5173/mock/register');
    await page.getByLabel('Email').fill(email);
    await page.getByLabel('Display name').fill('First User');
    await page.getByLabel('Password', { exact: true }).fill('password123');
    await page.getByLabel('Confirm password').fill('password123');
    await page.getByRole('button', { name: 'Create account' }).click();
    await expect(page).toHaveURL(/\/mock\/lobby/);
    
    // Try to register with same email
    await page.goto('http://localhost:5173/mock/register');
    await page.getByLabel('Email').fill(email);
    await page.getByLabel('Display name').fill('Second User');
    await page.getByLabel('Password', { exact: true }).fill('password456');
    await page.getByLabel('Confirm password').fill('password456');
    await page.getByRole('button', { name: 'Create account' }).click();
    
    // Should show error
    await expect(page.getByText(/already registered/i)).toBeVisible();
    
    // Should stay on register page
    await expect(page).toHaveURL(/\/mock\/register/);
    
    // Take screenshot of error state
    await page.screenshot({ path: 'e2e/screenshots/register-error-duplicate.png', fullPage: true });
  });

  test('Register validation: should show error for password mismatch', async ({ page }) => {
    await page.goto('http://localhost:5173/mock/register');
    
    await page.getByLabel('Email').fill('test@example.com');
    await page.getByLabel('Display name').fill('Test User');
    await page.getByLabel('Password', { exact: true }).fill('password123');
    await page.getByLabel('Confirm password').fill('differentpassword');
    await page.getByRole('button', { name: 'Create account' }).click();
    
    // Should show error
    await expect(page.getByText(/do not match/i)).toBeVisible();
    
    // Take screenshot of validation error
    await page.screenshot({ path: 'e2e/screenshots/register-error-mismatch.png', fullPage: true });
  });

  test('Login error: should show error for wrong password', async ({ page }) => {
    // Register a user first
    const timestamp = Date.now();
    const email = `wrongpass${timestamp}@example.com`;
    await page.goto('http://localhost:5173/mock/register');
    await page.getByLabel('Email').fill(email);
    await page.getByLabel('Display name').fill('Wrong Pass User');
    await page.getByLabel('Password', { exact: true }).fill('correctpassword');
    await page.getByLabel('Confirm password').fill('correctpassword');
    await page.getByRole('button', { name: 'Create account' }).click();
    await expect(page).toHaveURL(/\/mock\/lobby/);
    
    // Try to login with wrong password
    await page.goto('http://localhost:5173/mock/login');
    await page.getByLabel('Email').fill(email);
    await page.getByLabel('Password').fill('wrongpassword');
    await page.getByRole('button', { name: 'Login' }).click();
    
    // Should show error
    await expect(page.getByText(/invalid/i)).toBeVisible();
    
    // Should stay on login page
    await expect(page).toHaveURL(/\/mock\/login/);
    
    // Take screenshot of error state
    await page.screenshot({ path: 'e2e/screenshots/login-error-wrong-password.png', fullPage: true });
  });

  test('Login error: should show error for non-existent user', async ({ page }) => {
    await page.goto('http://localhost:5173/mock/login');
    
    await page.getByLabel('Email').fill('nonexistent@example.com');
    await page.getByLabel('Password').fill('password123');
    await page.getByRole('button', { name: 'Login' }).click();
    
    // Should show error
    await expect(page.getByText(/invalid/i)).toBeVisible();
    
    // Take screenshot of error state
    await page.screenshot({ path: 'e2e/screenshots/login-error-nonexistent.png', fullPage: true });
  });
});
