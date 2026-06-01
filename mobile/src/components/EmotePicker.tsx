import { useState } from 'react'
import { Modal, Pressable, Text, View } from 'react-native'
import { emotes } from '../game/emotes'

// Native port of web/src/components/EmotePicker.tsx. A floating button that
// opens a tray of emotes; selecting one calls onSelect and closes the tray.
// The tray is a small transparent modal so it floats above the table.
export function EmotePicker({ onSelect }: { onSelect: (id: string) => void }) {
  const [open, setOpen] = useState(false)

  const choose = (id: string) => {
    onSelect(id)
    setOpen(false)
  }

  return (
    <>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel="Open emotes"
        onPress={() => setOpen(true)}
        className="size-12 items-center justify-center rounded-full border border-spade-cream/15 bg-spade-bg/90 active:opacity-80"
      >
        <Text className="text-xl">😊</Text>
      </Pressable>

      <Modal visible={open} transparent animationType="fade" onRequestClose={() => setOpen(false)}>
        <Pressable className="flex-1 justify-end bg-black/40 p-4" onPress={() => setOpen(false)}>
          <Pressable
            accessibilityRole="menu"
            accessibilityLabel="Emotes"
            className="flex-row flex-wrap justify-center gap-2 rounded-spade-lg border border-spade-cream/12 bg-spade-bg/95 p-3"
            onPress={(e) => e.stopPropagation()}
          >
            {emotes.map((emote) => {
              const isWord = /[a-zA-Z]/.test(emote.glyph)
              return (
                <Pressable
                  key={emote.id}
                  accessibilityRole="menuitem"
                  accessibilityLabel={emote.label}
                  onPress={() => choose(emote.id)}
                  className="h-14 w-16 items-center justify-center rounded-spade-md border border-transparent bg-spade-cream/5 active:border-spade-gold/40"
                >
                  <Text className={isWord ? 'text-sm font-semibold text-spade-gold-light' : 'text-2xl'}>
                    {emote.glyph}
                  </Text>
                </Pressable>
              )
            })}
          </Pressable>
        </Pressable>
      </Modal>
    </>
  )
}
