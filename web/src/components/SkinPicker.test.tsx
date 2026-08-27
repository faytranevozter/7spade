import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'
import type { CatalogSkinDto, OwnedSkinDto } from '../api/skins'
import { useSkinAsset } from '../hooks/useSkinAsset'
import { SkinPicker } from './SkinPicker'

vi.mock('../hooks/useSkinAsset', () => ({
  useSkinAsset: vi.fn(() => 'https://assets.test/skins/player-card-backgrounds/gilded-seat.svg'),
}))

afterEach(() => {
  cleanup()
  vi.mocked(useSkinAsset).mockReturnValue('https://assets.test/skins/player-card-backgrounds/gilded-seat.svg')
})

const gildedSeat: OwnedSkinDto = {
  id: 'skin-player-card',
  skin_type: 'player_card_background',
  name: 'Gilded Seat',
  description: 'A gilded felt backdrop for your in-game player card.',
  asset_key: 'skins/player-card-backgrounds/gilded-seat.svg',
  display_order: 40,
  source: 'starter',
  equipped: false,
}

const otherSkins: OwnedSkinDto[] = [
  {
    ...gildedSeat,
    id: 'skin-profile-background',
    skin_type: 'profile_background',
    name: 'Gilded Table',
    asset_key: 'skins/backgrounds/gilded-table.svg',
  },
  {
    ...gildedSeat,
    id: 'skin-avatar-frame',
    skin_type: 'avatar_frame',
    name: 'Gold Spade Frame',
    asset_key: 'skins/frames/gold-spade.svg',
  },
  {
    ...gildedSeat,
    id: 'skin-display-picture',
    skin_type: 'display_picture',
    name: 'Ace of Spades',
    asset_key: 'skins/display-pictures/ace-spade.svg',
  },
]

const lockedSkin: CatalogSkinDto = {
  ...gildedSeat,
  id: 'skin-locked',
  name: 'Veteran Seat',
  unlock_rules: [{ rule_type: 'minimum_level', minimum_level: 10 }],
}

const gildedSeatCatalog: CatalogSkinDto = {
  ...gildedSeat,
  unlock_rules: [],
}

test('shows owned and locked cosmetics with client-authored requirements', () => {
  render(
    <SkinPicker
      skins={[gildedSeat]}
      catalog={[gildedSeatCatalog, lockedSkin]}
      busyType={null}
      onEquip={vi.fn()}
      onUnequip={vi.fn()}
    />,
  )

  const owned = screen.getByLabelText('Gilded Seat cosmetic')
  expect(owned).toHaveTextContent('Owned')
  const ownedDetails = within(owned).getByLabelText('Unlock details for Gilded Seat').closest('details') as HTMLDetailsElement
  expect(ownedDetails.open).toBe(false)
  fireEvent.click(within(owned).getByLabelText('Unlock details for Gilded Seat'))
  expect(ownedDetails.open).toBe(true)
  expect(ownedDetails).toHaveTextContent('Available to every player as a starter cosmetic')

  const locked = screen.getByLabelText('Veteran Seat cosmetic')
  expect(locked).toHaveTextContent('Locked')
      expect(locked).toHaveClass('bg-spade-bg/25')
	expect(locked).not.toHaveClass('opacity-70')
  const details = within(locked).getByText('Unlock details').closest('details') as HTMLDetailsElement
  expect(details.open).toBe(false)
  fireEvent.click(within(locked).getByText('Unlock details'))
  expect(details.open).toBe(true)
  expect(details).toHaveTextContent('Reach player level 10')
  expect(within(locked).queryByRole('button')).not.toBeInTheDocument()
})

test('raises an open popover above skin cards but below the sticky navbar', async () => {
  render(
    <SkinPicker skins={[]} catalog={[lockedSkin]} busyType={null} onEquip={vi.fn()} onUnequip={vi.fn()} />,
  )

  const cosmetic = screen.getByLabelText('Veteran Seat cosmetic')
  fireEvent.click(within(cosmetic).getByText('Unlock details'))
  await waitFor(() => expect(cosmetic).toHaveClass('z-10'))
  expect(cosmetic).not.toHaveClass('z-50')
})

test('shows event provenance for an owned event-only cosmetic outside the catalog', () => {
  const eventSkin: OwnedSkinDto = {
    ...gildedSeat,
    id: 'event-skin',
    name: 'Event Laurel Frame',
    source: 'event:playwright-test-event',
  }

  render(
    <SkinPicker
      skins={[eventSkin]}
      catalog={[]}
      busyType={null}
      onEquip={vi.fn()}
      onUnequip={vi.fn()}
    />,
  )

  const cosmetic = screen.getByLabelText('Event Laurel Frame cosmetic')
  fireEvent.click(within(cosmetic).getByLabelText('Unlock details for Event Laurel Frame'))
  expect(cosmetic).toHaveTextContent('Unlocked during Playwright Test Event')
  expect(cosmetic).not.toHaveTextContent('Unlock requirement unavailable')
})

test('shows the check-in requirement for an unowned event cosmetic', () => {
  const eventSkin: CatalogSkinDto = {
    ...lockedSkin,
    id: 'event-skin-locked',
    name: 'Event Laurel Frame',
    unlock_rules: [{
      rule_type: 'event_check_in_count',
      event_check_in_count: 1,
      event: {
        slug: 'playwright-test-event',
        name: 'Playwright Test Event',
        starts_at: '2026-08-01T00:00:00Z',
        ends_at: '2026-09-01T00:00:00Z',
      },
    }],
  }

  render(
    <SkinPicker
      skins={[]}
      catalog={[eventSkin]}
      busyType={null}
      onEquip={vi.fn()}
      onUnequip={vi.fn()}
    />,
  )

  const cosmetic = screen.getByLabelText('Event Laurel Frame cosmetic')
  fireEvent.click(within(cosmetic).getByText('Unlock details'))
  expect(cosmetic).toHaveTextContent('Check in on 1 event day')
  expect(cosmetic).not.toHaveTextContent('Unlock requirement unavailable')
})

test('lays out complex unlock details as separate conditions and event availability', () => {
  const eventChallenge: CatalogSkinDto = {
    ...lockedSkin,
    id: 'event-challenge',
    name: 'Precision Victor Frame',
    unlock_rules: [{
      rule_type: 'game_condition',
      name: 'precision-victor',
      conditions: [
        { metric: 'is_winner', operator: 'eq', value: 'true' },
        { metric: 'penalty', operator: 'lte', value: '5' },
      ],
      event: {
        slug: 'summer-table',
        name: 'Summer Table',
        starts_at: '2026-08-01T00:00:00Z',
        ends_at: '2026-09-01T00:00:00Z',
      },
    }],
  }

  render(
    <SkinPicker skins={[]} catalog={[eventChallenge]} busyType={null} onEquip={vi.fn()} onUnequip={vi.fn()} />,
  )

  const cosmetic = screen.getByLabelText('Precision Victor Frame cosmetic')
  fireEvent.click(within(cosmetic).getByText('Unlock details'))
  expect(within(cosmetic).getByText('How to unlock')).toBeInTheDocument()
  expect(within(cosmetic).getByText('Win a completed game')).toBeInTheDocument()
  expect(within(cosmetic).getByText('Finish with at most 5 penalty points')).toBeInTheDocument()
  expect(within(cosmetic).getByText('Event exclusive')).toBeInTheDocument()
  expect(within(cosmetic).getByText('Summer Table')).toBeInTheDocument()
  expect(within(cosmetic).queryByText(/Win a completed game and Finish/)).not.toBeInTheDocument()
})

test('closes unlock details when clicking outside the popover', async () => {
  render(
    <div>
      <button type="button">Outside</button>
      <SkinPicker skins={[]} catalog={[lockedSkin]} busyType={null} onEquip={vi.fn()} onUnequip={vi.fn()} />
    </div>,
  )

  const cosmetic = screen.getByLabelText('Veteran Seat cosmetic')
  const details = within(cosmetic).getByText('Unlock details').closest('details') as HTMLDetailsElement
  fireEvent.click(within(cosmetic).getByText('Unlock details'))
  expect(details.open).toBe(true)
  fireEvent.pointerDown(screen.getByRole('button', { name: 'Outside' }))
  await waitFor(() => expect(details.open).toBe(false))
})

test('keeps the second popover open when switching directly between skins', async () => {
  const secondSkin: CatalogSkinDto = { ...lockedSkin, id: 'skin-locked-2', name: 'Champion Seat' }
  render(
    <SkinPicker skins={[]} catalog={[lockedSkin, secondSkin]} busyType={null} onEquip={vi.fn()} onUnequip={vi.fn()} />,
  )

  const first = screen.getByLabelText('Veteran Seat cosmetic')
  const second = screen.getByLabelText('Champion Seat cosmetic')
  const firstDetails = within(first).getByText('Unlock details').closest('details') as HTMLDetailsElement
  const secondDetails = within(second).getByText('Unlock details').closest('details') as HTMLDetailsElement

  fireEvent.click(within(first).getByText('Unlock details'))
  expect(firstDetails.open).toBe(true)
  fireEvent.click(within(second).getByText('Unlock details'))

  await waitFor(() => {
    expect(firstDetails.open).toBe(false)
    expect(secondDetails.open).toBe(true)
  })
})

test('preserves a usable default preview when an asset is missing', () => {
  vi.mocked(useSkinAsset).mockReturnValue(null)

  render(
    <SkinPicker
      skins={[gildedSeat]}
      busyType={null}
      onEquip={vi.fn()}
      onUnequip={vi.fn()}
    />,
  )

  const preview = screen.getByLabelText('Gilded Seat player card preview')
  expect(preview).toBeInTheDocument()
  expect(preview.querySelector('img')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Equip' })).toBeEnabled()
})

test('offers player card backgrounds with a card preview and equip action', () => {
  const onEquip = vi.fn()

  render(
    <SkinPicker
      skins={[gildedSeat]}
      busyType={null}
      onEquip={onEquip}
      onUnequip={vi.fn()}
    />,
  )

  const category = screen.getByLabelText('Player card backgrounds')
  expect(within(category).getByLabelText('Gilded Seat player card preview')).toBeInTheDocument()
  fireEvent.click(within(category).getByRole('button', { name: 'Equip' }))
  expect(onEquip).toHaveBeenCalledWith(gildedSeat)
})

test('equipped player card background can be reset to default', () => {
  const onUnequip = vi.fn()

  render(
    <SkinPicker
      skins={[{ ...gildedSeat, equipped: true }]}
      busyType={null}
      onEquip={vi.fn()}
      onUnequip={onUnequip}
    />,
  )

  const cosmetic = screen.getByLabelText('Gilded Seat cosmetic')
  fireEvent.click(within(cosmetic).getByRole('button', { name: 'Use default' }))
  expect(onUnequip).toHaveBeenCalledWith('player_card_background')
})

test('previews each cosmetic in the aspect ratio of its destination', () => {
  render(
    <SkinPicker
      skins={[...otherSkins, gildedSeat]}
      busyType={null}
      onEquip={vi.fn()}
      onUnequip={vi.fn()}
    />,
  )

  expect(screen.getByLabelText('Gilded Table profile background preview')).toHaveClass('aspect-[20/7]')
  expect(screen.getByLabelText('Gilded Seat player card preview')).toHaveClass('aspect-[6/7]', 'w-full')
  expect(screen.getByLabelText('Gold Spade Frame avatar frame preview')).toHaveClass('aspect-square', 'w-full')
  expect(screen.getByLabelText('Ace of Spades display picture preview')).toHaveClass('aspect-square', 'w-full')
  expect(screen.getByLabelText('Gilded Table profile background preview').parentElement).toHaveClass('h-auto')
  expect(screen.queryByText('GT')).not.toBeInTheDocument()
  expect(screen.queryByText('GS')).not.toBeInTheDocument()
  expect(screen.queryByText('AV')).not.toBeInTheDocument()
  for (const preview of [
    'Gilded Table profile background preview',
    'Gilded Seat player card preview',
    'Gold Spade Frame avatar frame preview',
    'Ace of Spades display picture preview',
  ]) {
    expect(screen.getByLabelText(preview).querySelector('img')).toHaveClass('object-contain')
  }
})

test('sizes cosmetic cards for their preview shape', () => {
  render(
    <SkinPicker
      skins={[...otherSkins, gildedSeat]}
      busyType={null}
      onEquip={vi.fn()}
      onUnequip={vi.fn()}
    />,
  )

  expect(screen.getByLabelText('Gilded Table cosmetic')).toHaveClass('max-w-xl')
  expect(screen.getByLabelText('Gilded Seat cosmetic')).toHaveClass('max-w-56')
  expect(screen.getByLabelText('Gold Spade Frame cosmetic')).toHaveClass('max-w-56')
  expect(screen.getByLabelText('Ace of Spades cosmetic')).toHaveClass('max-w-56')
  expect(screen.getAllByRole('region').map((region) => region.getAttribute('aria-label'))).toEqual([
    'Profile backgrounds',
    'Player card backgrounds',
    'Avatar frames',
    'Display pictures',
  ])
})
