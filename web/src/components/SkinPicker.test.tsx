import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
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
  unlock_requirement: 'Reach player level 10',
}

test('shows owned and locked cosmetics with server-authored requirements', () => {
  render(
    <SkinPicker
      skins={[gildedSeat]}
      catalog={[gildedSeat, lockedSkin]}
      busyType={null}
      onEquip={vi.fn()}
      onUnequip={vi.fn()}
    />,
  )

  expect(screen.getByLabelText('Gilded Seat cosmetic')).toHaveTextContent('Owned')
  const locked = screen.getByLabelText('Veteran Seat cosmetic')
  expect(locked).toHaveTextContent('Locked')
  expect(locked).toHaveTextContent('Reach player level 10')
  expect(within(locked).queryByRole('button')).not.toBeInTheDocument()
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
