import type { LoginStreakResponse } from '../api/loginProgress'
import { Button } from './Button'
import { Modal } from './Modal'

export function DailyLoginXPModal({ reward, onClose }: { reward: LoginStreakResponse; onClose: () => void }) {
  return (
    <Modal
      eyebrow="Daily reward"
      title="XP earned"
      description={`Day ${reward.current_streak} streak reward`}
      onClose={onClose}
      footer={<Button onClick={onClose}>Continue</Button>}
    >
      <div className="rounded-spade-lg border border-spade-gold/30 bg-spade-gold/10 p-5 text-center">
        <p className="font-mono text-4xl font-semibold text-spade-gold-light">+{reward.xp_delta} XP</p>
        <p className="mt-2 text-sm text-spade-gray-2">{reward.xp_after.toLocaleString()} total XP · Level {reward.level}</p>
      </div>
    </Modal>
  )
}
