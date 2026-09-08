# Review Since v0.11.1

Original review baseline: `v0.11.1...7ca8a85`. Finding numbers refer to that
review, not GitHub issue numbers.

## Fixed Findings

| Finding | Status | Resolution |
| --- | --- | --- |
| P1 #1: invalid migration 043 | Fixed | Corrected missing VALUES comma. |
| P1 #2: invalid Tests workflow | Fixed | Corrected Build step indentation. |
| P1 #3: rejected edge spectator broadcasts | Fixed | Pending spectators excluded from broadcasts until owner admission; fatal rejection closes the socket. |
| P1 #4: unsafe MFA replacement | Fixed | Handler rejects re-enrollment; atomic persistence guard preserves verified factors during races. |
| P1 #5: achievement normalization | Fixed | Normalize actual request rules before validation and persistence; create/update regression coverage. |
| P1 #6: unavailable skin revisions block progression | Fixed | Publication guard, explicit current-revision pinning, unavailable-revision filtering, and starter trigger migration 045. |
| P1 #7: per-frame access dependency | Fixed | Periodic revocation checks replace synchronous per-message access checks. |
| P1 #8: API-less WS panic | Fixed | Avoid typed-nil application-controls interface. |
| P1 #9: expired admin access tokens | Fixed | Shared bounded recovery, concurrent/late-401 handling, logout race protection, and stable form state. |
| P1 #10: invitation acceptance CPU exhaustion | Fixed | Rate-limit before lookup/hash, reject invalid tokens cheaply, preserve transactional acceptance. |
| P2 #13: retroactive grants pin obsolete revisions | Fixed | Backfill selects the enabled current revision instead of any enabled revision. |
| P2 #23: skin asset URL omitted from image workflows | Fixed | Both image workflows pass `VITE_SKIN_ASSETS_URL` to the web Docker build. |
| P2 #24: admin API missing S3 configuration in Compose | Fixed | Compose forwards all supported `S3_*` settings to `admin-api`. |
| P2 #25: admin CSP blocks asset and blob previews | Fixed | Admin nginx renders one configured asset source into `img-src` at startup and permits `blob:` previews without a wildcard. |
| P2 #26: admin nginx rejects supported uploads | Fixed | The admin API proxy accepts 6 MiB requests, covering a 5 MiB file plus multipart overhead. |

The seven reported admin-web App.test.tsx failures are resolved. Updated tests
retain behavior assertions for room inspection, skin creation, metadata updates,
mutation errors, event rules, and asset publication. Metadata-only edits now omit
unchanged unlock rules.

## Verification

- Player API: 312 tests passed.
- WS: 195 tests passed with the race detector.
- Admin API: 69 tests passed with the race detector.
- Admin web: 92 tests passed; lint and production build passed.
- Skin grant and starter-trigger integration tests passed on disposable PostgreSQL 16.
- Workflow YAML parsing and git diff whitespace checks passed.
- Deployment wiring checks passed: Compose interpolation, nginx template rendering/config validation, and Docker build-arg inspection.
- Migration 045 has not been applied to the existing application database.
- Non-blocking admin bundle-size warning remains.

## P2 Plan

Implement in the following order, with regression tests and separate focused
commits. Re-check each finding against current code before changing it.

### 1. Deployment Wiring

- [x] #24: Forward S3 configuration to admin-api in Compose.
- [x] #26: Allow supported upload sizes plus multipart overhead in admin nginx.
- [x] #25: Permit blob previews and the configured asset origin in admin CSP.
- [x] #23: Pass VITE_SKIN_ASSETS_URL through both image-build workflows.

Verified Compose interpolation with dummy secrets and S3 settings, parsed both
workflow files with a YAML parser, built both frontend images, confirmed the web
bundle contains the configured asset URL, and ran `nginx -T` against the rendered
admin template with configured and empty asset origins. The rendered CSP contains
`blob:` and only the configured asset source; nginx's upload envelope is 6 MiB.
`actionlint` was not available in the local environment.

### 2. WS State And Audit

- [ ] #17: Propagate access enforcement through spectator-first room restoration.
- [ ] #18: Report remote players' logical connection state in live inspection.
- [ ] #22: Align audit filters with all persisted outcomes, including historical values.
- [ ] #21: Atomically audit MFA credential changes without recording secrets.

Verify restore/reconnect/suspend and multi-replica inspection flows; exercise audit
listing/export and rollback when audit insertion fails.

### 3. Frontend State

- [ ] #19: Keep event datetime inputs local and convert validated values on submission.
- [ ] #20: Invalidate equipped-skin cache after changes and revalidate public entries.

Test non-UTC and empty datetime inputs, repeated edits, equip/unequip, mounted
consumers, and navigation without a full reload.

### 4. Persistence And Rewards

- [ ] #11: Persist skin creation rules and conditions in the creation transaction.
- [ ] #12: Enforce event eligibility and provenance for retroactive level grants.
- [ ] #14: Preserve previous event revision eligibility for delayed game results.
- [ ] #15: Reject achievement idempotency-key reuse for a different operation.
- [ ] #16: Serialize concurrent identical entitlement retries and replay the result.

Use real PostgreSQL tests for persistence, rollback, revision windows, and
concurrent requests. #14 may require an additive migration to preserve historical
rule/revision associations; retain already stored reward provenance.

## Completion Gate

Run all affected suites and deployment smoke tests, update this checklist with
verified results, and distinguish environment limitations from successful checks.
There are 11 open P2 findings; #13 was resolved during P1 grant hardening and
#23-26 were resolved by deployment wiring hardening.
