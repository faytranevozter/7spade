import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { CardStack } from '../components/CardStack'
import { GameBoard } from '../components/GameBoard'
import { Modal } from '../components/Modal'
import { PlayerAvatar } from '../components/PlayerAvatar'
import { SceneShell } from '../components/SceneShell'
import { ToastStack } from '../components/ToastStack'
import { hand, noMoveHand, players, reconnectPlayers, toasts } from '../data/mockGame'
import type { Card, Player, Toast } from '../types'

export type GameScenario =
  | 'playable'
  | 'start-seven'
  | 'no-valid-move'
  | 'invalid-move'
  | 'timer-warning'
  | 'opponent-turn'
  | 'opponent-played-card'
  | 'opponent-passed'
  | 'disconnected-player-bot'
  | 'reconnect'
  | 'round-ending'

type ScenarioConfig = {
  title: string
  eyebrow: string
  status: string
  timer: string
  players: Player[]
  cards: Card[]
  handInteractive: boolean
  handMeta: string
  toasts: Toast[]
  sideTitle: string
  sideBody: string
  action?: 'play' | 'penalty' | 'waiting' | 'reconnect' | 'round-ending'
}

const sevenStartHand: Card[] = hand.map((card) => ({
  ...card,
  playable: card.rank === '7',
  selected: card.rank === '7' && card.suit === 'Clubs',
}))

const disabledHand: Card[] = hand.map((card) => ({
  ...card,
  playable: false,
  selected: false,
}))

const invalidHand: Card[] = hand.map((card) => ({
  ...card,
  playable: card.rank === '6' && card.suit === 'Spades',
  selected: card.rank === 'Q' && card.suit === 'Diamonds',
}))

const endingHand: Card[] = [
  { rank: 'K', suit: 'Clubs', playable: true, selected: true },
]

const activeOpponentPlayers = players.map((player) => ({
  ...player,
  active: player.name === 'Budi',
}))

const passedPlayers = activeOpponentPlayers.map((player) => ({
  ...player,
  faceDownCount: player.name === 'Budi' ? player.faceDownCount + 1 : player.faceDownCount,
}))

const scenarioMap: Record<GameScenario, ScenarioConfig> = {
  playable: {
    title: 'My turn - playable card',
    eyebrow: 'Game scene',
    status: 'Your turn',
    timer: '00:18',
    players,
    cards: hand,
    handInteractive: true,
    handMeta: '13 cards - playable cards are highlighted',
    toasts: [toasts[0], toasts[1]],
    sideTitle: 'Choose a legal move',
    sideBody: 'Play a highlighted card to extend an existing sequence, or pass only when the engine says no move is legal.',
    action: 'play',
  },
  'start-seven': {
    title: 'My turn - start a suit',
    eyebrow: 'Game scene',
    status: 'Start new suit',
    timer: '00:24',
    players,
    cards: sevenStartHand,
    handInteractive: true,
    handMeta: '13 cards - any 7 can open its suit row',
    toasts: [{ tone: 'info', title: 'New suit available', body: 'Play a 7 to start another sequence' }],
    sideTitle: 'Open the Clubs row',
    sideBody: 'The board has a closed suit row. A 7 in hand can create the center card for that suit.',
    action: 'play',
  },
  'no-valid-move': {
    title: 'My turn - no valid move',
    eyebrow: 'Game scene',
    status: 'Penalty required',
    timer: '00:21',
    players,
    cards: noMoveHand,
    handInteractive: true,
    handMeta: '11 cards - select one card to place face-down',
    toasts: [{ tone: 'warn', title: 'No valid move', body: 'Choose one card to place face-down as penalty' }],
    sideTitle: 'Place a penalty card',
    sideBody: 'This static state replaces the old standalone face-down selection page.',
    action: 'penalty',
  },
  'invalid-move': {
    title: 'My turn - invalid move',
    eyebrow: 'Game scene',
    status: 'Your turn',
    timer: '00:16',
    players,
    cards: invalidHand,
    handInteractive: true,
    handMeta: '13 cards - selected card cannot be played here',
    toasts: [toasts[3]],
    sideTitle: 'Board unchanged',
    sideBody: 'Invalid moves are shown only to the player who attempted them. The static board remains in the previous state.',
    action: 'play',
  },
  'timer-warning': {
    title: 'My turn - timer warning',
    eyebrow: 'Game scene',
    status: '10 seconds left',
    timer: '00:09',
    players,
    cards: hand,
    handInteractive: true,
    handMeta: '13 cards - timer pressure state',
    toasts: [toasts[1]],
    sideTitle: 'Move before auto-play',
    sideBody: 'The warning state prepares the player for an automatic deterministic move when the timer expires.',
    action: 'play',
  },
  'opponent-turn': {
    title: 'Opponent turn',
    eyebrow: 'Game scene',
    status: "Budi's turn",
    timer: '00:27',
    players: activeOpponentPlayers,
    cards: disabledHand,
    handInteractive: false,
    handMeta: '13 cards - hand is visible but disabled',
    toasts: [toasts[2]],
    sideTitle: 'Waiting for Budi',
    sideBody: 'Board cards stand still and the local hand is not interactive while another player is active.',
    action: 'waiting',
  },
  'opponent-played-card': {
    title: 'Opponent played a card',
    eyebrow: 'Game scene',
    status: 'Card played',
    timer: '00:30',
    players: activeOpponentPlayers,
    cards: disabledHand,
    handInteractive: false,
    handMeta: '13 cards - waiting for next turn',
    toasts: [{ tone: 'success', title: 'Budi played 9 Diamonds', body: 'The Diamonds row was extended upward' }],
    sideTitle: 'Latest move',
    sideBody: 'A compact success state confirms the opponent action and updates counts without changing route context.',
    action: 'waiting',
  },
  'opponent-passed': {
    title: 'Opponent placed penalty',
    eyebrow: 'Game scene',
    status: 'Penalty placed',
    timer: '00:30',
    players: passedPlayers,
    cards: disabledHand,
    handInteractive: false,
    handMeta: '13 cards - waiting for next turn',
    toasts: [{ tone: 'warn', title: 'Budi placed face-down', body: 'Penalty count increased to 1 card' }],
    sideTitle: 'Face-down count changed',
    sideBody: 'The card value stays hidden until scoring, but the player-facing count updates immediately.',
    action: 'waiting',
  },
  'disconnected-player-bot': {
    title: 'Disconnected player bot',
    eyebrow: 'Game scene',
    status: 'Bot active',
    timer: '00:12',
    players: reconnectPlayers,
    cards: disabledHand,
    handInteractive: false,
    handMeta: '13 cards - another player is bot-controlled',
    toasts: [{ tone: 'info', title: 'Budi disconnected', body: 'Auto-play will handle their turns until reconnect' }],
    sideTitle: 'Game continues',
    sideBody: 'The disconnected avatar is dimmed and marked Bot while the deterministic auto-play path is active.',
    action: 'waiting',
  },
  reconnect: {
    title: 'Reconnect to room',
    eyebrow: 'Game scene',
    status: 'Rejoining',
    timer: '00:18',
    players: reconnectPlayers,
    cards: disabledHand,
    handInteractive: false,
    handMeta: '13 cards - snapshot restored after reconnect',
    toasts: [{ tone: 'info', title: 'Connection restored', body: 'Review the current board before resuming play' }],
    sideTitle: 'Resume Meja Santai #1',
    sideBody: 'The reconnect scene shows room context, current turn, and a single rejoin action.',
    action: 'reconnect',
  },
  'round-ending': {
    title: 'Round ending',
    eyebrow: 'Game scene',
    status: 'Last card',
    timer: '00:07',
    players: players.map((player) => ({ ...player, cardsLeft: player.name === 'Fahrur' ? 1 : 0 })),
    cards: endingHand,
    handInteractive: true,
    handMeta: '1 card - scoring starts after this move',
    toasts: [{ tone: 'info', title: 'Scoring soon', body: 'All hands are almost empty' }],
    sideTitle: 'Final move',
    sideBody: 'After the last hand is empty, penalty cards are revealed and the results route takes over.',
    action: 'round-ending',
  },
}

export function GameScenarioPage({ scenario }: { scenario: GameScenario }) {
  const config = scenarioMap[scenario]
  const timerWidth = config.timer === '00:09' || config.timer === '00:07' ? 'w-1/5 bg-spade-red' : 'w-3/5 bg-spade-gold-light'

  return (
    <SceneShell
      title={config.title}
      eyebrow={config.eyebrow}
      action={(
        <>
          <Badge tone={config.status.includes('Penalty') || config.status.includes('10 seconds') ? 'danger' : 'playing'}>
            {config.status}
          </Badge>
          <span className="rounded-spade-pill border border-spade-gold-light/40 bg-spade-gold/15 px-3 py-1 font-mono text-xs text-spade-gold-light">
            {config.timer}
          </span>
        </>
      )}
    >
      <div className="grid gap-4">
        <div className="h-2 overflow-hidden rounded-full bg-spade-bg/70">
          <div className={`h-full rounded-full ${timerWidth}`} />
        </div>

        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {config.players.map((player) => (
            <PlayerAvatar key={player.name} player={player} />
          ))}
        </div>

        <GameBoard />

        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
          <CardStack cards={config.cards} interactive={config.handInteractive} meta={config.handMeta} />
          <div className="grid content-start gap-3 rounded-spade-lg border border-spade-cream/10 bg-spade-bg/50 p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <h2 className="text-lg font-medium">{config.sideTitle}</h2>
                <p className="mt-1 text-sm leading-5 text-spade-gray-2">{config.sideBody}</p>
              </div>
              <Badge tone="waiting">Static</Badge>
            </div>
            <ScenarioActions action={config.action} />
          </div>
        </div>

        <ToastStack toasts={config.toasts} />
      </div>
    </SceneShell>
  )
}

function ScenarioActions({ action }: { action?: ScenarioConfig['action'] }) {
  if (action === 'penalty') {
    return (
      <Modal
        title="Place face-down penalty"
        eyebrow="No valid move"
        description="Select one card from your hand. Its value will count as penalty at the end of the game."
        footer={(
          <>
            <Button variant="secondary">Review cards</Button>
            <Button variant="danger">Place face-down</Button>
          </>
        )}
        tone="danger"
      >
        <div className="rounded-spade-md border border-spade-red/20 bg-spade-red/10 p-3 text-sm text-spade-cream">
          J of Diamonds selected <span className="font-mono text-spade-gold-light">penalty: 11 pts</span>
        </div>
      </Modal>
    )
  }

  if (action === 'reconnect') {
    return (
      <div className="grid gap-2">
        <Button>Reconnect to room</Button>
        <Button variant="secondary">Return to lobby</Button>
      </div>
    )
  }

  if (action === 'waiting') {
    return (
      <div className="grid gap-2">
        <Button disabled>Play card</Button>
        <Button variant="secondary" disabled>Pass turn</Button>
      </div>
    )
  }

  if (action === 'round-ending') {
    return (
      <div className="grid gap-2">
        <Button>Play final card</Button>
        <Button variant="secondary">Review penalties</Button>
      </div>
    )
  }

  return (
    <div className="grid grid-cols-2 gap-2">
      <Button>Play card</Button>
      <Button variant="secondary">Pass turn</Button>
      <Button variant="ghost">Copy link</Button>
      <Button variant="danger">Forfeit</Button>
    </div>
  )
}
