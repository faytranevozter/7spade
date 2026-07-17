import { useEffect, useMemo, useRef, useState } from 'react'
import { Badge } from './Badge'
import { Button } from './Button'
import { CardFace } from './CardFace'
import { GameBoard } from './GameBoard'
import { Modal } from './Modal'
import { buildBoardRows } from '../hooks/useGameSocket'
import {
  TUTORIAL_STEPS,
  applyGuidedPlay,
  cardMatchesRequired,
  formatCardShort,
  formatTutorialActionHint,
  isFaceDownRequired,
  isTutorialTarget,
  normalizeRequiredPlays,
  writeTutorialStatus,
  type TutorialStep,
} from '../game/tutorial'
import type { Card, CloseMethod } from '../types'
import type { WireBoardRange } from '../hooks/useGameSocket'

type SceneState = {
  board: Record<string, WireBoardRange>
  closedSuits: string[]
  aceCloseMethod?: CloseMethod
  hand: Card[]
}

function sceneFromStep(step: TutorialStep): SceneState {
  return {
    board: step.board,
    closedSuits: [...(step.closedSuits ?? [])],
    aceCloseMethod: step.aceCloseMethod,
    hand: step.hand.map((c) => ({ ...c })),
  }
}

type TutorialOverlayProps = {
  onClose: (result: 'completed' | 'skipped') => void
  onStartPractice: () => void
}

export function TutorialOverlay({ onClose, onStartPractice }: TutorialOverlayProps) {
  const [stepIndex, setStepIndex] = useState(0)
  const [selectedFaceDown, setSelectedFaceDown] = useState<Card | null>(null)
  const [closePrompt, setClosePrompt] = useState<Card | null>(null)
  const [playedThisStep, setPlayedThisStep] = useState(false)
  const [autoPlayed, setAutoPlayed] = useState(false)
  const [scene, setScene] = useState<SceneState>(() => sceneFromStep(TUTORIAL_STEPS[0]))
  const [timerLeft, setTimerLeft] = useState<number | null>(null)
  const playedRef = useRef(false)

  const step = TUTORIAL_STEPS[stepIndex]
  const isLast = stepIndex === TUTORIAL_STEPS.length - 1
  const boardRows = useMemo(
    () => buildBoardRows(scene.board, scene.closedSuits, scene.aceCloseMethod),
    [scene],
  )
  const actionHint = !playedThisStep ? formatTutorialActionHint(step) : null

  const goToStep = (index: number) => {
    const next = TUTORIAL_STEPS[index]
    setStepIndex(index)
    setScene(sceneFromStep(next))
    setSelectedFaceDown(null)
    setClosePrompt(null)
    setPlayedThisStep(false)
    setAutoPlayed(false)
    playedRef.current = false
    setTimerLeft(next.showTimer ? (next.timerSeconds ?? 8) : null)
  }

  const advance = () => {
    if (isLast) {
      writeTutorialStatus('completed')
      onClose('completed')
      return
    }
    goToStep(stepIndex + 1)
  }

  const skip = () => {
    writeTutorialStatus('skipped')
    onClose('skipped')
  }

  const finishAndPractice = () => {
    writeTutorialStatus('completed')
    onClose('completed')
    onStartPractice()
  }

  const canNextWithoutPlay = !step.requiredPlay || playedThisStep

  const applyRequiredPlay = (
    activeStep: TutorialStep,
    card: Card,
    faceDown: boolean,
    aceMethod?: CloseMethod,
    fromAuto = false,
  ) => {
    if (!activeStep.requiredPlay) return
    if (!cardMatchesRequired(card, activeStep.requiredPlay)) return
    if (isFaceDownRequired(activeStep.requiredPlay) !== faceDown) return
    if (playedRef.current) return
    playedRef.current = true
    const next = applyGuidedPlay(activeStep, card, { faceDown, aceMethod })
    setScene({
      board: next.board,
      closedSuits: next.closedSuits,
      aceCloseMethod: next.aceCloseMethod,
      hand: next.hand,
    })
    setPlayedThisStep(true)
    setAutoPlayed(fromAuto)
    setTimerLeft(null)
  }

  // Live demo countdown for the turn-timer step. When it hits 0, auto-play the
  // first legal guided card (same idea as the server auto-play bot). Stops as
  // soon as the learner plays (playedThisStep).
  useEffect(() => {
    if (!step.showTimer || playedThisStep) return

    const activeStep = step
    // Start from the value goToStep already wrote (or step default).
    let remaining = activeStep.timerSeconds ?? 8
    const id = window.setInterval(() => {
      remaining -= 1
      if (remaining <= 0) {
        window.clearInterval(id)
        setTimerLeft(0)
        if (!playedRef.current && activeStep.requiredPlay) {
          const option = normalizeRequiredPlays(activeStep.requiredPlay)[0]
          const card =
            activeStep.hand.find((c) => c.rank === option.rank && c.suit === option.suit) ??
            ({ rank: option.rank, suit: option.suit, playable: true } as Card)
          applyRequiredPlay(activeStep, card, Boolean(option.faceDown), undefined, true)
        }
        return
      }
      setTimerLeft(remaining)
    }, 1000)

    return () => window.clearInterval(id)
    // Re-arm only for this step; playedThisStep ends the interval via cleanup.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentional: bound to step identity
  }, [step.id, stepIndex, playedThisStep])

  const handleHandClick = (card: Card) => {
    if (playedThisStep || playedRef.current) return

    // Only guided targets accept clicks (even in face-down mode).
    if (step.requiredPlay && !isTutorialTarget(card, step.requiredPlay)) return

    if (step.faceDownMode) {
      setSelectedFaceDown(card)
      return
    }

    if (!card.playable && !isTutorialTarget(card, step.requiredPlay)) return

    if (card.aceClose) {
      const { canLow, canHigh } = card.aceClose
      if (canLow && canHigh) {
        setClosePrompt(card)
        return
      }
      applyRequiredPlay(step, card, false, canLow ? 'low' : 'high')
      return
    }

    applyRequiredPlay(step, card, false)
  }

  const confirmFaceDown = () => {
    if (!selectedFaceDown) return
    applyRequiredPlay(step, selectedFaceDown, true)
    setSelectedFaceDown(null)
  }

  const confirmAceClose = (method: CloseMethod) => {
    if (!closePrompt) return
    applyRequiredPlay(step, closePrompt, false, method)
    setClosePrompt(null)
  }

  // Preserve original playable flags from the step hand (for static green rings).
  const handPlayableByKey = useMemo(() => {
    const map = new Map<string, boolean>()
    for (const card of step.hand) {
      map.set(`${card.suit}:${card.rank}`, Boolean(card.playable))
    }
    return map
  }, [step])

  const visibleHand = scene.hand.map((card) => {
    const isTarget = !playedThisStep && isTutorialTarget(card, step.requiredPlay)
    const isLegal = Boolean(handPlayableByKey.get(`${card.suit}:${card.rank}`))
    // Face-down guided target: show as playable so it gets a ring + pulse.
    const showPlayable = !playedThisStep && (isTarget || isLegal)
    return {
      ...card,
      // Green ring for any legal/target card; gold pulse only on guided targets.
      playable: showPlayable,
      dimmed: !playedThisStep && !showPlayable,
      selected:
        step.faceDownMode &&
        selectedFaceDown?.rank === card.rank &&
        selectedFaceDown?.suit === card.suit,
    }
  })

  const timerUrgent = timerLeft !== null && timerLeft <= 3 && !playedThisStep
  const autoPlayLabel =
    autoPlayed && step.requiredPlay
      ? formatCardShort(
          normalizeRequiredPlays(step.requiredPlay)[0].rank,
          normalizeRequiredPlays(step.requiredPlay)[0].suit,
        )
      : null

  return (
    <Modal
      title={step.title}
      eyebrow={`Tutorial · ${stepIndex + 1} / ${TUTORIAL_STEPS.length}`}
      size="wide"
      onClose={skip}
      footer={
        <>
          <Button type="button" variant="secondary" onClick={skip}>
            Skip tutorial
          </Button>
          {step.isSummary ? (
            <>
              <Button type="button" variant="secondary" onClick={advance}>
                Done
              </Button>
              <Button type="button" onClick={finishAndPractice}>
                Start practice
              </Button>
            </>
          ) : (
            <Button type="button" onClick={advance} disabled={!canNextWithoutPlay}>
              {isLast ? 'Finish' : 'Next'}
            </Button>
          )}
        </>
      }
    >
      <div className="grid gap-4" data-testid="tutorial-overlay">
        <p className="text-sm leading-6 text-spade-gray-2">{step.body}</p>

        {actionHint ? (
          <div
            className="rounded-spade-md border border-spade-gold/45 bg-spade-gold/15 px-3 py-2.5"
            data-testid="tutorial-action-hint"
            role="status"
          >
            <p className="text-[11px] font-semibold uppercase tracking-[0.14em] text-spade-gold-light">
              Your move
            </p>
            <p className="mt-1 text-sm font-medium text-spade-cream">{actionHint}</p>
          </div>
        ) : null}

        {step.showTimer ? (
          <div
            className={`flex items-center justify-between rounded-spade-md border px-3 py-2 ${
              timerUrgent
                ? 'border-spade-red/50 bg-spade-red/15'
                : 'border-spade-gold/30 bg-spade-gold/10'
            }`}
            data-testid="tutorial-timer"
          >
            <span
              className={`text-xs font-medium uppercase tracking-[0.12em] ${
                timerUrgent ? 'text-[#ffb4a8]' : 'text-spade-gold-light'
              }`}
            >
              {playedThisStep && autoPlayed ? 'Time expired — auto-play' : 'Your turn'}
            </span>
            <Badge tone={timerUrgent || (playedThisStep && autoPlayed) ? 'danger' : 'waiting'}>
              {playedThisStep
                ? autoPlayed
                  ? '0s'
                  : 'Played'
                : `${timerLeft ?? step.timerSeconds ?? 8}s`}
            </Badge>
          </div>
        ) : null}

        <GameBoard rows={boardRows} />

        {visibleHand.length > 0 || (step.requiredPlay && !playedThisStep) ? (
          <div>
            <p className="mb-2 text-xs font-medium uppercase tracking-[0.12em] text-spade-gray-3">
              {playedThisStep
                ? 'Your hand'
                : step.faceDownMode
                  ? 'No legal plays — place the pulsing card face down.'
                  : step.showTimer
                    ? 'Play a pulsing card before the timer hits 0.'
                    : 'Green = legal play. Pulsing gold = click this one.'}
            </p>
            {visibleHand.length > 0 ? (
              <div className="flex flex-wrap gap-2" data-testid="tutorial-hand">
                {visibleHand.map((card) => {
                  const isTarget = !playedThisStep && isTutorialTarget(card, step.requiredPlay)
                  return (
                    <CardFace
                      key={`${card.suit}-${card.rank}`}
                      card={card}
                      size="md"
                      interactive={!playedThisStep && isTarget}
                      onClick={() => handleHandClick(card)}
                      animationClassName={isTarget ? 'anim-tutorial-target' : ''}
                    />
                  )
                })}
              </div>
            ) : null}
            {step.faceDownMode && selectedFaceDown && !playedThisStep ? (
              <div className="mt-3 flex justify-end">
                <Button type="button" size="sm" onClick={confirmFaceDown}>
                  Place face down
                </Button>
              </div>
            ) : null}
            {playedThisStep ? (
              <p className="mt-2 text-xs text-spade-green-light" role="status" data-testid="tutorial-play-status">
                {autoPlayed && autoPlayLabel
                  ? `Time ran out — auto-played ${autoPlayLabel}. Press Next to continue.`
                  : 'Nice — press Next to continue.'}
              </p>
            ) : null}
          </div>
        ) : null}

        {step.scorePreview ? (
          <div
            className="overflow-hidden rounded-spade-md border border-spade-cream/10"
            data-testid="tutorial-scores"
          >
            <table className="w-full text-left text-sm">
              <thead className="bg-spade-bg/60 text-xs uppercase tracking-[0.12em] text-spade-gray-3">
                <tr>
                  <th className="px-3 py-2 font-medium">Player</th>
                  <th className="px-3 py-2 font-medium">Penalty</th>
                  <th className="px-3 py-2 font-medium">Result</th>
                </tr>
              </thead>
              <tbody>
                {step.scorePreview.map((row) => (
                  <tr
                    key={row.name}
                    className={row.me ? 'bg-spade-gold/10 text-spade-cream' : 'text-spade-gray-2'}
                  >
                    <td className="px-3 py-2">{row.name}</td>
                    <td className="px-3 py-2 font-mono">{row.penalty}</td>
                    <td className="px-3 py-2">{row.winner ? 'Winner' : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}

        {closePrompt ? (
          <AceClosePrompt card={closePrompt} onChoose={confirmAceClose} onCancel={() => setClosePrompt(null)} />
        ) : null}
      </div>
    </Modal>
  )
}

function AceClosePrompt({
  card,
  onChoose,
  onCancel,
}: {
  card: Card
  onChoose: (method: CloseMethod) => void
  onCancel: () => void
}) {
  return (
    <div
      className="rounded-spade-md border border-spade-gold/35 bg-spade-bg/70 p-3"
      data-testid="tutorial-ace-close"
      role="group"
      aria-label="Choose ace close method"
    >
      <p className="text-sm text-spade-cream">
        Close {card.rank} of {card.suit} — low (after 2, Ace = 1) or high (after King, Ace = 14)?
      </p>
      <p className="mt-1 text-xs text-spade-gray-3">
        The first Ace close locks this method for every suit this game.
      </p>
      <div className="mt-3 flex flex-wrap gap-2">
        <Button type="button" size="sm" onClick={() => onChoose('low')}>
          Close low
        </Button>
        <Button type="button" size="sm" onClick={() => onChoose('high')}>
          Close high
        </Button>
        <Button type="button" size="sm" variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </div>
  )
}
