import { chromium } from '/home/yeyingxingchen/Xinghai/gpt-account-tools/node_modules/playwright-core/index.mjs';

async function main() {
  const browser = await chromium.launch({ 
    headless: true,
    executablePath: '/usr/bin/chromium',
    args: ['--no-sandbox', '--disable-setuid-sandbox']
  });
  
  const context = await browser.newContext({ locale: 'en-US' });
  const page = await context.newPage();
  
  const logs = [];
  page.on('console', msg => logs.push(`[console.${msg.type()}] ${msg.text()}`));
  page.on('pageerror', err => logs.push(`[PAGE_ERROR] ${err.message}\n${err.stack || ''}`));
  page.on('request', req => {
    const url = req.url();
    if (url.includes('/api/')) logs.push(`[REQ ${req.method()}] ${url}`);
  });
  page.on('response', res => {
    const url = res.url();
    if (url.includes('/api/')) logs.push(`[RES ${res.status()}] ${url}`);
  });
  page.on('requestfailed', req => logs.push(`[REQ_FAILED] ${req.url()} ${req.failure()?.errorText}`));
  
  const BASE = 'http://127.0.0.1:3000';
  
  await page.goto(BASE, { waitUntil: 'networkidle', timeout: 15000 });
  
  const loginResp = await page.evaluate(async () => {
    const res = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: 'quota-test-1@example.com', password: 'password123' })
    });
    return await res.json();
  });
  
  if (!loginResp.token) {
    console.log('Login failed');
    await browser.close();
    return;
  }
  
  await context.addCookies([{
    name: 'xinghai.admin-token',
    value: loginResp.token,
    domain: '127.0.0.1',
    path: '/',
    httpOnly: false,
    secure: false,
    sameSite: 'Lax'
  }]);
  
  await page.goto(`${BASE}/console/keys`, { waitUntil: 'networkidle', timeout: 15000 });
  await page.waitForSelector('tr:has-text("Test Quota Key")', { timeout: 10000 });
  await page.waitForTimeout(1500);
  
  console.log('=== Clicking edit on test key ===');
  const row = page.locator('tr', { hasText: 'Test Quota Key' }).first();
  const moreBtn = row.locator('button').last();
  await moreBtn.click();
  await page.waitForTimeout(800);
  
  // Wait for the quota GET request explicitly
  const quotaPromise = page.waitForRequest(req => req.url().includes('/quota'), { timeout: 5000 }).catch(() => null);
  
  const editItem = page.locator('[role="menuitem"]', { hasText: 'Edit' }).first();
  await editItem.click();
  
  const quotaReq = await quotaPromise;
  console.log('Quota request seen:', quotaReq ? quotaReq.url() : 'NO QUOTA REQUEST');
  
  await page.waitForTimeout(2000);
  
  // Dump all dialogs
  const dialogs = page.locator('[role="dialog"]');
  const count = await dialogs.count();
  console.log('Dialog count:', count);
  for (let i = 0; i < count; i++) {
    const html = await dialogs.nth(i).evaluate(el => el.outerHTML);
    console.log(`\n--- Dialog ${i} HTML ---`);
    console.log(html);
  }
  
  console.log('\n=== Network/console logs ===');
  for (const line of logs) {
    console.log(line);
  }
  
  await browser.close();
}

main().catch(err => {
  console.error('Error:', err);
  process.exit(1);
});