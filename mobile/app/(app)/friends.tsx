import { View } from 'react-native'
import { SceneShell } from '../../src/components/SceneShell'
import { FriendsPanel } from '../../src/components/FriendsPanel'
import { useAuth } from '../../src/hooks/useAuth'

// Full-screen Friends view, reusing the same FriendsPanel shown on the lobby.
// Useful as a deep-link target and a dedicated tab for managing requests.
export default function FriendsScreen() {
  const { token } = useAuth()
  return (
    <View className="flex-1 bg-spade-bg">
      <SceneShell title="Friends" eyebrow="Your players">
        <FriendsPanel token={token} refreshNonce={0} />
      </SceneShell>
    </View>
  )
}
