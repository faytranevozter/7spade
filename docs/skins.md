# Adding Skins

This guide covers three workflows:

1. **Publish through the admin app** for normal operational additions and new asset revisions.
2. **Seed a skin in a migration** when it must ship with the application.
3. **Add a skin type** (a new cosmetic category). This also changes database constraints, backend validation, frontend types, previews, and a runtime rendering destination.

## Existing Skin Types

| Type | Asset path | Ratio | Example size | Runtime destination |
|---|---|---:|---:|---|
| `profile_background` | `skins/backgrounds/` | `20:7` | `1200x420` | Profile hero |
| `player_card_background` | `skins/player-card-backgrounds/` | `6:7` | `240x280` | Active-game opponent card |
| `avatar_frame` | `skins/frames/` | `1:1` | `200x200` | Overlay around an avatar |
| `display_picture` | `skins/display-pictures/` | `1:1` | `200x200` | Avatar image, circularly cropped at runtime |

The embedded seed uploader currently uses SVG. The admin publishing workflow
accepts PNG, JPEG, WebP, and SVG and records the content type on each revision.

Keep important artwork inside the center safe area. Runtime backgrounds use cover-style placement and may lose edges when their destination differs slightly from the source ratio. Avatar frames should have a transparent center and transparent outer corners. Display pictures should keep important artwork within a circle.

The Cosmetics picker displays only the source image. It uses `object-contain`, so the complete asset remains visible without cropping at mobile and desktop widths.

## Catalog API

Authenticated clients load the active catalog from `GET /skins`. The response keeps the `{ "skins": [...] }` envelope and returns `unlock_rules` as ordered, typed domain data rather than pre-rendered English text. Rules are alternatives (OR); conditions within a `game_condition` rule are cumulative (AND). Event-bound rules include the event slug, name, start, and end timestamps.

The API includes only enabled, `catalog_visible` skins. A skin backed exclusively by event rules appears only while at least one associated event is enabled and active. PostgreSQL time is authoritative for that window. Skins are ordered by `skin_type`, `display_order`, and ID; rules by name; and challenge conditions by creation time and ID.

Ownership is revision-pinned. Catalog responses use the skin's current asset,
while owned and equipped responses use the revision granted to that user. A
disabled skin or revision is omitted and cannot be equipped. Owned/equip
responses return `unlock_rules: null` because those queries do not reload rules.

Event-bound unlock rules are revision-pinned independently from skin assets.
Each publication associates the rule with an immutable `event_versions` row;
later event edits and republication append another association rather than
replacing historical eligibility. Game-result rewards use the game's finish time
to choose the associated published revision whose `[starts_at, ends_at)` window
contains that time. Level rewards use the causal XP-award time and persist the
selected event ID and revision. This allows delayed results to grant the reward
that was valid when play occurred without opening future event windows early.

## Publish Through Admin

Use the admin Skins screen for routine creation and publication. An admin with
`skins.manage` can create metadata, upload or register an asset, and publish an
immutable revision; `skins.entitlements` controls exceptional grants and
revocations. Use a new asset key for every revision. Disabling a revision hides
ownership pinned to it, so treat disable as an operational revocation rather
than an asset-edit mechanism.

Skin creation commits metadata, unlock rules, challenge conditions, historical
event association (when the event is already published), and audit data in one
transaction. The response contains persisted rule IDs and submitted conditions;
a failure in any part rolls back the whole skin.

`web/src/components/SkinPicker.tsx` owns the human-readable requirement wording. Update its formatter and tests when adding a rule type, metric, or operator. This separation keeps the API machine-readable and allows presentation or localization to evolve independently.

## Seed a Skin in a Migration

### 1. Create the asset

Place the SVG under the embedded asset tree:

```text
services/admin-api/cmd/skinassets/assets/<asset-key>
```

For example:

```text
services/admin-api/cmd/skinassets/assets/skins/player-card-backgrounds/gilded-seat.svg
```

Use a new asset key for every published revision. Uploaded objects have `Cache-Control: public, max-age=31536000, immutable`; replacing bytes at an existing key can leave clients on the old image for a year.

Completion criterion: the SVG view box matches the category ratio and contains no external fonts, images, scripts, or remote references.

### 2. Embed the asset

Add a `go:embed` declaration and map entry in `services/admin-api/cmd/skinassets/main.go`:

```go
//go:embed assets/skins/player-card-backgrounds/example.svg
var examplePlayerCard []byte

// In the assets map:
"skins/player-card-backgrounds/example.svg": examplePlayerCard,
```

The map key must exactly match the database `asset_key`. The uploader uploads every map entry, not only the new one.

Completion criterion: `go test ./cmd/skinassets` compiles the embedded asset.

### 3. Add an append-only migration

Create the next numbered file under `services/api/internal/database/migrations/`. Never edit an already-applied migration to add catalog data.

Use a stable UUID and a unique asset key:

```sql
INSERT INTO skins (
    id, skin_type, name, description, asset_key,
    is_starter, display_order, catalog_visible
)
VALUES (
    'REPLACE-WITH-A-STABLE-UUID',
    'player_card_background',
    'Example Skin',
    'Short player-facing description.',
    'skins/player-card-backgrounds/example.svg',
    FALSE,
    50,
    TRUE
)
ON CONFLICT (id) DO NOTHING;
```

`display_order` orders skins within a type. Category order is defined separately in `web/src/components/SkinPicker.tsx`.

Create the initial immutable revision before granting ownership. Its
`asset_key` and content type must match the seeded asset:

```sql
INSERT INTO skin_revisions (id, skin_id, version, asset_key, content_type)
VALUES (
    'REPLACE-WITH-A-STABLE-REVISION-UUID',
    'REPLACE-WITH-THE-SKIN-UUID',
    1,
    'skins/player-card-backgrounds/example.svg',
    'image/svg+xml'
)
ON CONFLICT (id) DO NOTHING;
```

#### Starter skins

Set `is_starter = TRUE` when every persisted registered account should own the
skin. The existing `grant_starter_skins()` trigger grants every starter skin to
future `users` rows; it does not filter disabled skins, and guests do not have a
persisted user row. Keep disabled rows out of the starter set. Backfill existing
registered users in the same migration:

```sql
INSERT INTO user_skins (user_id, skin_id, skin_revision_id, source)
SELECT u.id,
       'REPLACE-WITH-THE-SKIN-UUID',
       'REPLACE-WITH-A-STABLE-REVISION-UUID',
       'starter'
FROM users u
ON CONFLICT (user_id, skin_id) DO NOTHING;
```

Starter ownership does not auto-equip a skin. For non-starter skins, add the intended award or purchase path that inserts into `user_skins`; a catalog row alone is not owned by anyone.

Completion criterion: the migration inserts the catalog row and grants exactly the intended ownership without creating an equipped row.

### 4. Upload to S3-compatible storage

Set the variables documented in `services/admin-api/.env.example`, including:

```text
S3_ENDPOINT
S3_ACCESS_KEY_ID
S3_SECRET_ACCESS_KEY
S3_BUCKET
S3_REGION
S3_PUBLIC_URL
S3_USE_PATH_STYLE
SKIN_ASSET_CORS_ALLOWED_ORIGINS
```

Run from `services/admin-api` so the command reads `services/admin-api/.env`. Set `SKIN_ASSET_CORS_ALLOWED_ORIGINS` to a comma-separated list of player and admin frontend origins to configure bucket CORS. The command only needs storage configuration, not database or authentication secrets:

```bash
go run ./cmd/skinassets
```

The uploader first attempts to configure bucket CORS. A CORS `403` warning is non-fatal when every object upload succeeds: the frontend service worker supports opaque public responses. An object upload error is fatal.

Verify the public object without credentials:

```bash
PUBLIC_URL="https://assets.example.com"
ASSET_KEY="skins/player-card-backgrounds/example.svg"
curl -fsSI "$PUBLIC_URL/$ASSET_KEY"
```

Expected headers include `200`, `Content-Type: image/svg+xml`, and the immutable cache policy.

Completion criterion: the public URL returns the new bytes and the uploader logged the exact asset key as uploaded.

### 5. Verify the UI

An additional skin in an existing type appears automatically in My Profile → Cosmetics after ownership is granted. Check:

- The category is unchanged and skins follow `display_order`.
- The preview shows only the image at the category ratio.
- Equip and Use default update the selected skin.
- The runtime destination renders the equipped skin.
- Missing or failed assets retain the default appearance.
- Mobile and desktop previews have no cropping or horizontal overflow.

Run:

```bash
cd web
npm test -- --run src/components/SkinPicker.test.tsx
npm run lint
npm run build
```

Run `go test ./...` from `services/api` when the migration, repository, or API behavior changed. Run `go test ./cmd/skinassets ./internal/storage` from `services/admin-api` when the uploader changed.

## Add a New Skin Type

Adding a type is a cross-layer contract change. Complete every item below.

### Database

Create a new migration that replaces the `skin_type` check constraints on both `skins` and `user_equipped_skins`. Deployed databases may use historical constraint names, so remove each known name with `DROP CONSTRAINT IF EXISTS` before adding the canonical constraint. See `031_skins.sql` for the current canonical type lists.

Seed and backfill the first skin only after the expanded constraints are active. Exercise the migration against PostgreSQL, including an existing schema and a clean schema.

### Backend

Update `services/api/internal/repository/skins.go`:

- Add a `SkinType...` constant.
- Include it in `IsSkinType`.
- Add the type to `skins_test.go`.

The existing skin handlers and ownership queries are type-agnostic after validation accepts the new value.

### Frontend

Update all category contract points:

- `web/src/api/skins.ts`: extend `SkinType`.
- `web/src/components/SkinPicker.tsx`: add the category in the intended order, set its card width, and render an image-only fixed-ratio preview.
- Runtime destination: resolve the equipped skin and render it with a safe default fallback.
- `web/src/components/SkinPicker.test.tsx`: cover category order, ratio, full image fitting, and equip/default actions.
- Destination tests: cover equipped, missing, failed, and inapplicable identities such as bots.

Use `useEquippedSkins` for public equipped-skin loading and `useSkinAsset` for asset resolution. This keeps requests deduplicated and preserves the service-worker cache behavior.

### Cache changes

Published asset keys are immutable. Prefer a new key when artwork changes. If an existing key must be replaced during development, increment `ASSET_CACHE_VERSION` in `web/src/hooks/useSkinAsset.ts`; keep the CacheStorage name synchronized with `web/public/skin-cache-sw.js`. Versioning the synthetic key avoids a controller-transition race between old and new service workers.

Completion criterion: database validation, Go validation, TypeScript types, Cosmetics, the runtime destination, tests, and the uploaded public asset all recognize the new type.

## Troubleshooting

### Migration violates a skin type check

The old check constraint is still active. Inspect the constraint name on both tables and update the new migration to drop historical and current names before inserting the new type. A failed migration is not recorded in `schema_migrations`, so restart the API after correcting it.

### Uploaded image does not change

The object URL is immutable and may exist in both HTTP and CacheStorage caches. Publish the changed artwork under a new asset key and update it through a migration. During development, increment `ASSET_CACHE_VERSION` and reload after the updated service worker activates.

### Synthetic `/__skin-assets__/...` URL returns 404

The hook and active service worker are using different CacheStorage names, or the response was not cached before the synthetic URL was returned. Keep the cache name synchronized, version the synthetic key, and reload once after a service-worker update.

### Asset is visible in Cosmetics but not in game/profile

Confirm the skin is equipped, the runtime payload includes a registered `user_id`, and the destination resolves the new type. Bots and users without an equipped skin intentionally render the default appearance.
