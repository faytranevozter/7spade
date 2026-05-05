# Seven Spade Web

React + TypeScript frontend for Seven Spade.

## Stack

- Vite
- React
- TypeScript
- Tailwind CSS v4.2 via `@tailwindcss/vite`

Tailwind should be installed through the Vite plugin and imported from the CSS entry:

```bash
npm install tailwindcss @tailwindcss/vite
```

```css
@import "tailwindcss";
```

## Design System

Frontend UI must follow `../design/design_system.html`.

Core tokens:

- `#0d1a12` Forest Night
- `#1a472a` Table Green
- `#2d7a46` Green Light
- `#c9922b` Gold
- `#f5c842` Gold Light
- `#f4ead5` Cream
- `#c0392b` Heart Red
- `#1e4080` Spade Blue
- `#1a1a1a` Card Black

Use DM Sans for UI text and DM Mono for room codes, counters, scores, and compact metadata.

The app should feel like a compact multiplayer card table: dark background, table-green game surfaces, cream cards, gold accents, restrained motion, and stable card/board dimensions. Avoid generic Vite template styling, marketing-page heroes, and unrelated palettes.

## Development

```bash
npm install
npm run dev
```

## Verification

```bash
npm run build
npm run lint
```

Run `npm test` when a test script exists or when frontend tests are added.
