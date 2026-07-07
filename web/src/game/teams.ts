export type TeamColor = {
  badge: string
  badgeActive: string
  border: string
  text: string
}

export const teamColors: TeamColor[] = [
  {
    badge: 'border-blue-400/30 bg-blue-500/12 text-blue-300 before:bg-blue-400',
    badgeActive: 'border-blue-400/40 bg-blue-500/8',
    border: 'border-blue-400/30',
    text: 'text-blue-300',
  },
  {
    badge: 'border-orange-400/30 bg-orange-500/12 text-orange-300 before:bg-orange-400',
    badgeActive: 'border-orange-400/40 bg-orange-500/8',
    border: 'border-orange-400/30',
    text: 'text-orange-300',
  },
  {
    badge: 'border-purple-400/30 bg-purple-500/12 text-purple-300 before:bg-purple-400',
    badgeActive: 'border-purple-400/40 bg-purple-500/8',
    border: 'border-purple-400/30',
    text: 'text-purple-300',
  },
]

export function getTeamColor(team: number): TeamColor {
  return teamColors[team] ?? teamColors[0]
}
