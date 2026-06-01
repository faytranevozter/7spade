import { useState } from 'react'
import { KeyboardAvoidingView, Platform, ScrollView, Text, TextInput, View } from 'react-native'
import { useRouter, Link } from 'expo-router'
import { Button } from '../../src/components/Button'
import { postGuest, postLogin, AuthApiError, type OAuthProvider } from '../../src/api/auth'
import { useAuth } from '../../src/hooks/useAuth'
import { useOAuth } from '../../src/hooks/useOAuth'

// Native port of web/src/pages/AuthPage.tsx. Guest + email login + OAuth.
// Navigation after login is handled by the root navigator's auth gate (it
// redirects to the lobby once the session is set), so screens just call login().
export default function AuthScreen() {
  const router = useRouter()
  const { login } = useAuth()
  const oauth = useOAuth()
  const [displayName, setDisplayName] = useState('')
  const [guestLoading, setGuestLoading] = useState(false)
  const [guestError, setGuestError] = useState<string | null>(null)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [loginLoading, setLoginLoading] = useState(false)
  const [loginError, setLoginError] = useState<string | null>(null)

  const getErrorMessage = (err: unknown) => {
    if (err instanceof AuthApiError) return err.message
    if (err instanceof Error) return err.message
    return 'An unexpected error occurred'
  }

  const handleGuest = async () => {
    setGuestError(null)
    setGuestLoading(true)
    try {
      const response = await postGuest(displayName)
      login(response.token, null)
    } catch (err) {
      setGuestError(getErrorMessage(err))
    } finally {
      setGuestLoading(false)
    }
  }

  const handleLogin = async () => {
    setLoginError(null)
    setLoginLoading(true)
    try {
      const response = await postLogin(email, password)
      login(response.jwt, response.refreshToken)
    } catch (err) {
      setLoginError(getErrorMessage(err))
    } finally {
      setLoginLoading(false)
    }
  }

  const handleOAuth = async (provider: OAuthProvider) => {
    const response = await oauth.signIn(provider)
    if (response) {
      login(response.jwt, response.refreshToken)
    }
  }

  return (
    <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} className="flex-1 bg-spade-bg">
      <ScrollView contentContainerClassName="grow justify-center px-4 py-8">
        <View className="mb-6 flex-row items-center justify-center gap-3">
          <View className="size-11 items-center justify-center rounded-spade-lg bg-spade-gold">
            <Text className="text-2xl text-[#1a0e00]">♠</Text>
          </View>
          <Text className="text-2xl font-bold text-spade-gold-light">SEVEN SPADE</Text>
        </View>

        <View className="rounded-spade-lg border border-spade-cream/10 bg-[#102316] p-6">
          <View className="mb-6 items-center">
            <Text className="text-2xl font-medium text-spade-cream">Take Your Seat</Text>
            <Text className="mt-1.5 text-sm text-spade-gray-2">Choose how you want to join the table.</Text>
          </View>

          <View>
            <Text className="text-lg font-medium text-spade-cream">Play as Guest</Text>
            <Text className="mt-1 text-sm text-spade-gray-2">No registration required. Jump straight into a casual room.</Text>
            <View className="mt-4 gap-3">
              <View className="gap-1.5">
                <Text className="text-xs font-medium uppercase text-spade-gray-2">Display name</Text>
                <TextInput
                  value={displayName}
                  onChangeText={setDisplayName}
                  placeholder="TableMaster99"
                  placeholderTextColor="#9c958966"
                  maxLength={50}
                  editable={!guestLoading}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
                />
              </View>
              {guestError ? <ErrorText>{guestError}</ErrorText> : null}
              <Button onPress={handleGuest} disabled={guestLoading || !displayName.trim()}>
                {guestLoading ? 'Joining...' : 'Continue'}
              </Button>
            </View>
          </View>

          <Divider label="Or" />

          <View>
            <Text className="text-lg font-medium text-spade-cream">Sign In</Text>
            <View className="mt-4 gap-4">
              <View className="gap-1.5">
                <Text className="text-xs font-medium uppercase text-spade-gray-2">Email</Text>
                <TextInput
                  value={email}
                  onChangeText={setEmail}
                  placeholder="player@example.com"
                  placeholderTextColor="#9c958966"
                  autoCapitalize="none"
                  keyboardType="email-address"
                  editable={!loginLoading}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
                />
              </View>
              <View className="gap-1.5">
                <Text className="text-xs font-medium uppercase text-spade-gray-2">Password</Text>
                <TextInput
                  value={password}
                  onChangeText={setPassword}
                  placeholder="Enter your password"
                  placeholderTextColor="#9c958966"
                  secureTextEntry
                  editable={!loginLoading}
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
                />
              </View>
              {loginError ? <ErrorText>{loginError}</ErrorText> : null}
              <Button onPress={handleLogin} disabled={loginLoading || !email || !password}>
                {loginLoading ? 'Signing in...' : 'Sign In'}
              </Button>
            </View>

            <Divider label="Or continue with" />

            {oauth.error ? <ErrorText>{oauth.error}</ErrorText> : null}

            <View className="mt-1 flex-row gap-3">
              <Button variant="secondary" className="flex-1" disabled={oauth.isLoading} onPress={() => handleOAuth('google')}>
                Google
              </Button>
              <Button variant="secondary" className="flex-1" disabled={oauth.isLoading} onPress={() => handleOAuth('github')}>
                GitHub
              </Button>
              <Button variant="secondary" className="flex-1" disabled={oauth.isLoading} onPress={() => handleOAuth('telegram')}>
                Telegram
              </Button>
            </View>

            <View className="mt-5 flex-row justify-center">
              <Text className="text-sm text-spade-gray-3">Don't have an account? </Text>
              <Link href="/(auth)/register" className="text-sm font-medium text-spade-gold">Register here</Link>
            </View>
          </View>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  )
}

function ErrorText({ children }: { children: string }) {
  return (
    <View className="rounded-spade-md border border-spade-red/50 bg-spade-red/10 px-3 py-2">
      <Text className="text-sm text-[#ffb4ab]">{children}</Text>
    </View>
  )
}

function Divider({ label }: { label: string }) {
  return (
    <View className="my-6 flex-row items-center gap-4">
      <View className="h-px flex-1 bg-spade-cream/10" />
      <Text className="font-mono text-xs uppercase text-spade-gray-3">{label}</Text>
      <View className="h-px flex-1 bg-spade-cream/10" />
    </View>
  )
}
