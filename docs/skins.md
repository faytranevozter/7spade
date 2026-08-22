# Adding Skins

This guide covers two workflows:

1. **Add a skin** to an existing category. This normally requires an asset, an upload entry, and a catalog migration.
2. **Add a skin type** (a new cosmetic category). This also changes database constraints, backend validation, frontend types, previews, and a runtime rendering destination.

## Existing Skin Types

| Type | Asset path | Ratio | Example size | Runtime destination |
|---|---|---:|---:|---|
| `profile_background` | `skins/backgrounds/` | `20:7` | `1200x420` | Profile hero |
| `player_card_background` | `skins/player-card-backgrounds/` | `6:7` | `240x280` | Active-game opponent card |
| `avatar_frame` | `skins/frames/` | `1:1` | `200x200` | Overlay around an avatar |
| `display_picture` | `skins/display-pictures/` | `1:1` | `200x200` | Avatar image, circularly cropped at runtime |

Assets currently use SVG. The uploader sends every embedded asset as `image/svg+xml`, so supporting another format also requires content-type handling in `services/api/cmd/skinassets/main.go`.

Keep important artwork inside the center safe area. Runtime backgrounds use cover-style placement and may lose edges when their destination differs slightly from the source ratio. Avatar frames should have a transparent center and transparent outer corners. Display pictures should keep important artwork within a circle.

The Cosmetics picker displays only the source image. It uses `object-contain`, so the complete asset remains visible without cropping at mobile and desktop widths.

## Add a Skin to an Existing Type

### 1. Create the asset

Place the SVG under the embedded asset tree:

```text
services/api/cmd/skinassets/assets/<asset-key>
```

For example:

```text
services/api/cmd/skinassets/assets/skins/player-card-backgrounds/gilded-seat.svg
```

Use a new asset key for every published revision. Uploaded objects have `Cache-Control: public, max-age=31536000, immutable`; replacing bytes at an existing key can leave clients on the old image for a year.

Completion criterion: the SVG view box matches the category ratio and contains no external fonts, images, scripts, or remote references.

### 2. Embed the asset

Add a `go:embed` declaration and map entry in `services/api/cmd/skinassets/main.go`:

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
    is_starter, display_order
)
VALUES (
    'REPLACE-WITH-A-STABLE-UUID',
    'player_card_background',
    'Example Skin',
    'Short player-facing description.',
    'skins/player-card-backgrounds/example.svg',
    FALSE,
    50
)
ON CONFLICT (id) DO NOTHING;
```

`display_order` orders skins within a type. Category order is defined separately in `web/src/components/SkinPicker.tsx`.

#### Starter skins

Set `is_starter = TRUE` when every account should own the skin. The existing `grant_starter_skins()` trigger grants all enabled starter skins to future user rows, including guests. Backfill existing rows in the same migration:

```sql
INSERT INTO user_skins (user_id, skin_id, source)
SELECT u.id, 'REPLACE-WITH-THE-SKIN-UUID', 'starter'
FROM users u
ON CONFLICT (user_id, skin_id) DO NOTHING;
```

Starter ownership does not auto-equip a skin. For non-starter skins, add the intended award or purchase path that inserts into `user_skins`; a catalog row alone is not owned by anyone.

Completion criterion: the migration inserts the catalog row and grants exactly the intended ownership without creating an equipped row.

### 4. Upload to S3-compatible storage

Set the variables documented in `services/api/.env.example`, including:

```text
S3_ENDPOINT
S3_ACCESS_KEY_ID
S3_SECRET_ACCESS_KEY
S3_BUCKET
S3_REGION
S3_PUBLIC_URL
S3_USE_PATH_STYLE
```

Run from `services/api` so `config.Load()` reads `services/api/.env`:

```bash
go run ./cmd/skinassets
```

The shared config loader also requires the normal API values such as `JWT_SECRET`, `DATABASE_URL`, and `INTERNAL_API_SECRET` to be present. Keep credentials in `.env` or the process environment; never put them in commands, logs, documentation, or commits.

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

Run `go test ./...` from `services/api` when the migration, repository, uploader, or API behavior changed.

## Add a New Skin Type

Adding a type is a cross-layer contract change. Complete every item below.

### Database

Create a new migration that replaces the `skin_type` check constraints on both `skins` and `user_equipped_skins`. Deployed databases have used more than one constraint name, so remove every historical name with `DROP CONSTRAINT IF EXISTS` before adding the canonical constraint. See `032_player_card_background.sql` for the compatibility pattern.

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
