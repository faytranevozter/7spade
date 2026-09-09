#!/usr/bin/env node
// Run: node scripts/admin-deployment-smoke.mjs (Node >=22, Docker, prebuilt images).
// Override ADMIN_API_IMAGE / ADMIN_WEB_IMAGE if needed. Never uses the local stack.
// HTTP loopback only: Secure flags are asserted, but deliberately not enforced by
// this same-origin cookie jar. Browser TLS/SameSite enforcement needs a separate test.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { createHmac, randomBytes, randomUUID } from 'node:crypto';
import { readdirSync, readFileSync } from 'node:fs';
import { setTimeout as sleep } from 'node:timers/promises';

const apiImage = process.env.ADMIN_API_IMAGE || '7spade-admin-api:deploy-test';
const webImage = process.env.ADMIN_WEB_IMAGE || '7spade-admin-web:deploy-test';
const network = `admin-smoke-${randomUUID()}`;
const containers = [];
let networkCreated = false;
const jwtSecret = randomBytes(32).toString('hex');
const credentials = { email: 'smoke@example.invalid', password: `Smoke-${randomBytes(24).toString('hex')}` };
const refreshName = 'admin_refresh_token', csrfName = 'admin_csrf_token';
const authPath = '/admin-api/auth';

function docker(args, input, allowedFailure = false) {
  const result = spawnSync('docker', args, { input, encoding: 'utf8', timeout: 120_000, maxBuffer: 8 << 20 });
  if (!allowedFailure && (result.error || result.status !== 0)) {
    throw new Error(`docker ${args[0]} failed: ${result.error?.message || result.stderr || result.stdout}`);
  }
  return result;
}
function start(name, image, args = []) {
  const id = docker(['create', '--pull=never', '--name', `${network}-${name}`, '--network', network,
    '--label', `admin-deployment-smoke=${network}`, ...args, image]).stdout.trim();
  containers.push(id);
  docker(['start', id]);
  return id;
}
function cleanup() {
  for (const id of containers.splice(0).reverse()) {
    const result = docker(['rm', '-f', '-v', id], undefined, true);
    if (result.status !== 0) { console.error(`Cleanup failed for own container ${id}`); process.exitCode = 1; }
  }
  if (networkCreated) {
    const result = docker(['network', 'rm', network], undefined, true);
    if (result.status !== 0) { console.error(`Cleanup failed for own network ${network}`); process.exitCode = 1; }
    networkCreated = false;
  }
}
for (const [signal, code] of [['SIGINT', 130], ['SIGTERM', 143]]) {
  process.once(signal, () => { cleanup(); process.exit(code); });
}
async function waitFor(label, check) {
  for (let n = 0; n < 60; n++) {
    try { if (await check()) return; } catch { /* Retry startup only, never test assertions. */ }
    await sleep(500);
  }
  throw new Error(`Timed out waiting for ${label}`);
}
const matchesPath = (path, scope) => path === scope || path.startsWith(scope.endsWith('/') ? scope : `${scope}/`);
const cookieHeader = (jar, path, readable = false) => [...jar.values()]
  .filter(c => matchesPath(path, c.path) && (!readable || !c.httpOnly)).map(c => `${c.name}=${c.value}`).join('; ');
function acceptCookies(jar, headers, clearing = false) {
  const lines = headers.getSetCookie();
  assert.equal(lines.length, 2, 'Expected refresh and CSRF Set-Cookie headers');
  const seen = new Set();
  for (const line of lines) {
    const [pair, ...parts] = line.split(';').map(s => s.trim());
    const split = pair.indexOf('=');
    const name = pair.slice(0, split), value = pair.slice(split + 1);
    assert.ok([refreshName, csrfName].includes(name), 'Unexpected cookie');
    seen.add(name);
    const attrs = Object.fromEntries(parts.map(p => { const i = p.indexOf('='); return i < 0 ? [p.toLowerCase(), true] : [p.slice(0, i).toLowerCase(), p.slice(i + 1)]; }));
    const path = name === refreshName ? authPath : '/';
    assert.equal(attrs.path, path, `${name} path`);
    assert.equal(attrs.secure, true, `${name} Secure`);
    assert.equal(Boolean(attrs.httponly), name === refreshName, `${name} HttpOnly`);
    assert.equal(attrs.samesite?.toLowerCase(), 'strict', `${name} SameSite`);
    assert.equal(attrs.domain, undefined, 'Cookies must be host-only');
    const key = `${name}:${path}`;
    if (clearing) {
      assert.equal(value, '');
      assert.ok(Number(attrs['max-age']) <= 0, `${name} deletion Max-Age`);
      jar.delete(key);
    } else {
      assert.ok(value && Number(attrs['max-age']) > 0, `${name} persistent value`);
      jar.set(key, { name, value, path, httpOnly: Boolean(attrs.httponly) });
    }
  }
  assert.equal(seen.size, 2);
}
async function request(base, path, status, { method = 'GET', token, jar = new Map(), csrf, body } = {}) {
  const headers = { Cookie: cookieHeader(jar, path) };
  if (token) headers.Authorization = `Bearer ${token}`;
  if (csrf !== undefined) headers['X-CSRF-Token'] = csrf;
  if (body && !(body instanceof FormData)) { headers['Content-Type'] = 'application/json'; body = JSON.stringify(body); }
  const response = await fetch(base + path, { method, headers, body, redirect: 'error', signal: AbortSignal.timeout(15_000) });
  const text = await response.text();
  assert.equal(response.status, status, `${method} ${path}: ${response.status}, expected ${status}; ${text.slice(0, 180)}`);
  return { headers: response.headers, data: text && response.headers.get('content-type')?.includes('application/json') ? JSON.parse(text) : text };
}
function totp(secret) {
  const bits = [...secret.replace(/=+$/, '').toUpperCase()].map(c => {
    const n = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'.indexOf(c); assert.ok(n >= 0); return n.toString(2).padStart(5, '0');
  }).join('');
  const key = Buffer.from(bits.match(/.{8}/g).map(b => parseInt(b, 2)));
  const counter = Buffer.alloc(8); counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30_000)));
  const hash = createHmac('sha1', key).update(counter).digest();
  return ((hash.readUInt32BE(hash[19] & 15) & 0x7fffffff) % 1_000_000).toString().padStart(6, '0');
}

try {
  assert.ok(Number(process.versions.node.split('.')[0]) >= 22, 'Node >=22 required');
  for (const image of [apiImage, webImage]) docker(['image', 'inspect', image]);
  if (docker(['image', 'inspect', 'postgres:16-alpine'], undefined, true).status !== 0) docker(['pull', 'postgres:16-alpine']);
  docker(['network', 'create', network]); networkCreated = true;
  const pg = start('postgres', 'postgres:16-alpine', ['--network-alias', 'postgres', '--tmpfs', '/var/lib/postgresql/data',
    '-e', 'POSTGRES_USER=smoke', '-e', 'POSTGRES_PASSWORD=disposable-smoke-only', '-e', 'POSTGRES_DB=smoke']);
  await waitFor('Postgres 16', () => docker(['exec', pg, 'pg_isready', '-h', '127.0.0.1', '-U', 'smoke', '-d', 'smoke'], undefined, true).status === 0);
  const directory = new URL('../services/api/internal/database/migrations/', import.meta.url);
  const migrations = readdirSync(directory).filter(f => f.endsWith('.sql')).sort();
  assert.ok(migrations.length > 0);
  // Match the API migrator's lexical order, atomic transaction, and version ledger.
  const sql = ['BEGIN;', 'CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT NOW());'];
  for (const file of migrations) {
    sql.push(readFileSync(new URL(file, directory), 'utf8'), `INSERT INTO schema_migrations(version) VALUES ('${file.replaceAll("'", "''")}');`);
  }
  sql.push('COMMIT;');
  docker(['exec', '-i', pg, 'psql', '-X', '-v', 'ON_ERROR_STOP=1', '-U', 'smoke', '-d', 'smoke'], sql.join('\n'));
  console.log(`PASS: fresh Postgres 16, ${migrations.length} ordered shared migrations`);
  const env = { DATABASE_URL: 'postgres://smoke:disposable-smoke-only@postgres:5432/smoke?sslmode=disable',
    APP_ENV: 'production', ADMIN_SECURE_COOKIES: 'true', ADMIN_JWT_SECRET: jwtSecret,
    ADMIN_MFA_ENCRYPTION_KEY: randomBytes(32).toString('hex'), ADMIN_FRONTEND_ORIGIN: 'http://127.0.0.1' };
  for (const link of ['METRICS', 'LOGS', 'TRACES', 'DEPLOYMENTS', 'RUNBOOK']) env[`OPERATIONS_${link}_URL`] = `https://example.invalid/${link.toLowerCase()}`;
  const api = start('api', apiImage, ['--network-alias', 'admin-api', ...Object.entries(env).flatMap(([k, v]) => ['-e', `${k}=${v}`])]);
  const bootstrap = ['exec', '-e', `ADMIN_BOOTSTRAP_EMAIL=${credentials.email}`, '-e', `ADMIN_BOOTSTRAP_PASSWORD=${credentials.password}`,
    '-e', 'ADMIN_BOOTSTRAP_NAME=Disposable Smoke Admin', api, '/bootstrap-admin'];
  docker(bootstrap);
  const repeat = docker(bootstrap, undefined, true);
  assert.notEqual(repeat.status, 0);
  assert.match(repeat.stderr + repeat.stdout, /bootstrap refused: an administrator already exists/);
  console.log('PASS: one-shot bootstrap and explicit repeat refusal');

  let base;
  for (const origin of ['', 'https://assets.example.invalid']) {
    const web = start(origin ? 'web-assets' : 'web', webImage, ['-p', '127.0.0.1::80', '-e', `SKIN_ASSETS_ORIGIN=${origin}`]);
    const address = docker(['port', web, '80/tcp']).stdout.trim();
    base = `http://${address}`;
    await waitFor('proxied API', async () => (await fetch(`${base}/admin-api/health`, { signal: AbortSignal.timeout(1000) })).ok);
    docker(['exec', web, 'nginx', '-t']);
    const root = await request(base, '/', 200);
    assert.match(root.data, /<html/i);
    const csp = root.headers.get('content-security-policy');
    for (const directive of ["default-src 'self'", "connect-src 'self'", "script-src 'self'", "img-src 'self' data: blob:"]) assert.ok(csp?.includes(directive), directive);
    assert.ok(!csp.includes('${'), 'Unrendered CSP placeholder');
    assert.equal(csp.includes('https://assets.example.invalid'), Boolean(origin));
    assert.equal((await request(base, '/admin-api/health', 200)).data.service, 'admin-api');
  }
  console.log('PASS: nginx -t, SPA, proxied health and CSP with empty/configured asset origins');
  const jar = new Map();
  let response = await request(base, `${authPath}/login`, 200, { method: 'POST', body: credentials });
  acceptCookies(jar, response.headers);
  let token = response.data.access_token;
  assert.ok(token);
  const originalJar = new Map(jar);
  assert.ok(cookieHeader(jar, `${authPath}/refresh`).includes(`${refreshName}=`));
  for (const path of ['/', '/admin-api/me', `${authPath}entication`]) assert.ok(!cookieHeader(jar, path).includes(`${refreshName}=`));
  assert.match(cookieHeader(jar, '/', true), /^admin_csrf_token=.+$/);
  const csrf = j => j.get(`${csrfName}:/`).value;
  await request(base, '/admin-api/me', 401);
  await request(base, '/admin-api/settings/daily-login', 200, { token });
  const setting = { enabled: false, reason: 'Disposable deployment smoke test' };
  response = await request(base, '/admin-api/settings/daily-login', 403, { method: 'PUT', token, body: setting });
  assert.equal(response.data.error, 'MFA required');
  // Player-shaped JWT with an otherwise valid signature must not cross the admin boundary.
  const claims = JSON.parse(Buffer.from(token.split('.')[1], 'base64url'));
  delete claims.iss; delete claims.aud;
  Object.assign(claims, { display_name: 'Smoke Player', is_guest: false });
  for (const secret of ['disposable-player-signing-key', jwtSecret]) {
    const unsigned = [ { alg: 'HS256', typ: 'JWT' }, claims ].map(v => Buffer.from(JSON.stringify(v)).toString('base64url')).join('.');
    const player = `${unsigned}.${createHmac('sha256', secret).update(unsigned).digest('base64url')}`;
    await request(base, '/admin-api/me', 401, { token: player });
  }
  for (const [path, method] of [['refresh', 'POST'], ['logout', 'DELETE']]) {
    for (const bad of [undefined, 'incorrect-csrf']) {
      response = await request(base, `${authPath}/${path}`, 403, { method, jar, csrf: bad });
      assert.equal(response.data.error, 'Invalid CSRF token');
    }
  }
  response = await request(base, `${authPath}/refresh`, 200, { method: 'POST', jar, csrf: csrf(jar) });
  acceptCookies(jar, response.headers);
  assert.notEqual(jar.get(`${refreshName}:${authPath}`).value, originalJar.get(`${refreshName}:${authPath}`).value, 'Refresh must rotate');
  assert.notEqual(csrf(jar), csrf(originalJar), 'CSRF cookie must rotate');
  assert.notEqual(response.data.access_token, token);
  await request(base, '/admin-api/me', 401, { token });
  token = response.data.access_token;
  await request(base, '/admin-api/me', 200, { token });
  console.log('PASS: cookie attributes/path jar, auth boundary, non-MFA read/mutation gate, CSRF and refresh rotation');

  const enrollment = await request(base, `${authPath}/mfa/enroll`, 200, { method: 'POST', token });
  response = await request(base, `${authPath}/mfa/confirm`, 200, { method: 'POST', token, body: { code: totp(enrollment.data.secret) } });
  assert.equal(response.data.recovery_codes.length, 8);
  const beforeLogout = new Map(jar);
  response = await request(base, `${authPath}/logout`, 204, { method: 'DELETE', jar, csrf: csrf(jar) });
  acceptCookies(jar, response.headers, true);
  assert.equal(jar.size, 0, 'Logout must delete the original cookie paths');
  await request(base, '/admin-api/me', 401, { token });
  for (const old of [beforeLogout, originalJar]) await request(base, `${authPath}/refresh`, 401, { method: 'POST', jar: old, csrf: csrf(old) });
  response = await request(base, `${authPath}/login`, 202, { method: 'POST', body: credentials });
  assert.equal(response.data.mfa_required, true);
  assert.ok(!response.data.access_token);
  assert.equal(response.headers.getSetCookie().length, 0);
  response = await request(base, `${authPath}/mfa/challenge`, 200, { method: 'POST', body: { challenge_token: response.data.challenge_token, code: totp(enrollment.data.secret) } });
  acceptCookies(jar, response.headers);
  token = response.data.access_token;
  await request(base, '/admin-api/settings/daily-login', 200, { method: 'PUT', token, body: setting });
  assert.equal((await request(base, '/admin-api/settings/daily-login', 200, { token })).data.enabled, false);
  const form = new FormData();
  form.append('file', new Blob([Buffer.alloc(5 << 20)], { type: 'image/png' }), 'smoke.png');
  response = await request(base, `/admin-api/skins/${randomUUID()}/assets`, 503, { method: 'POST', token, body: form });
  assert.equal(response.data.error, 'Asset storage unavailable', '5 MiB multipart must reach the API, not nginx 413');
  console.log('PASS: logout deletion/revocation, TOTP enrollment/login, MFA mutation and 5 MiB proxy body');
  console.log('PASS: admin deployment smoke. Caveat: HTTP jar checks paths/flags, not browser TLS/SameSite enforcement; no real asset storage or UI.');
} catch (error) {
  console.error(error);
  process.exitCode = 1;
} finally {
  cleanup();
}
