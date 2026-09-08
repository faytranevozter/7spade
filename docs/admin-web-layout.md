# Admin Web Layout

Authenticated admin pages use the Events and Achievements catalogs as the
layout reference. Shared framing lives in
`admin-web/src/components/AdminPage.tsx`.

## Page Frame

- Use `AdminPage` for authenticated page roots. It centers content at the
  shared `max-w-360` width; `AdminLayout` owns viewport padding.
- A root form that cannot be wrapped in a section should use
  `adminPageClassName` directly.
- Keep narrower reading or form columns inside the shared page frame rather
  than narrowing the header. Settings and Security use this pattern.

## Header

- Use `AdminPageHeader` with explicit `eyebrow`, `title`, `description`, and
  `actions` slots. Do not overload `children` to mean different slots.
- List and operation pages use the default `page` title size
  (`text-admin-hero`). Record and editor pages use `variant="detail"`.
- Header actions align right on wide screens and stack below the title below
  760px.
- Associate the page root and heading with `labelledBy` and `titleId` when the
  root is a semantic section.
- Keep visually attached summary strips and specialized avatar headers custom;
  shared components should not obscure those relationships.

## Spacing

- Header bottom padding: 32px (`pb-8`).
- Header-to-content and major section gap: 24px (`mt-6` / `gap-6`).
- Catalog and card grid gap: 16px (`gap-4`).
- Standard panel padding: 20px, reduced to 16px below 500px.
- Form field groups generally use 16px (`gap-4`).
- Panel footers use a top divider and 16px top padding (`pt-4`), unless a
  denser existing control group requires otherwise.

## Panels

- Use `AdminPanel` for ordinary content cards and forms. It provides the shared
  border, translucent surface, card shadow, radius, and responsive padding.
- Use `adminPanelClassName` when the panel itself must be a `form`, `section`,
  or link.
- Status-toned panels, attached summary strips, and sticky investigation
  sidebars may retain specialized styling.

## Responsive Rules

- Page headers stack at 760px.
- Catalog grids may collapse at their existing 1050px breakpoint.
- Keep component-specific table and investigation breakpoints when content
  requires them.
- Tables scroll inside their own containers; they must not widen the page.
- Primary mobile actions should fill the available width where the surrounding
  page already follows that convention.

## Exceptions

Login, MFA, and invitation acceptance are standalone access flows and do not
use the authenticated page frame. Any new authenticated page should start with
the shared components; a custom layout should be limited to the smallest area
that needs it.
