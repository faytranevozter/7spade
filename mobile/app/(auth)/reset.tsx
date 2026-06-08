import { useState } from 'react'
import { KeyboardAvoidingView, Platform, ScrollView, Text, TextInput, View } from 'react-native'
import { Link, useLocalSearchParams, useRouter } from 'expo-router'
import { Button } from '../../src/components/Button'
import { postResetPassword, AuthApiError } from '../../src/api/auth'

// Native port of web ResetPasswordPage. Reachable via the deep link
// sevenspade://reset?token=... (and the in-app stack). Reads the token from the
// route params, lets the user choose a new password, then routes back to sign in.
export default function ResetPasswordScreen() {
  const router = useRouter()
  const params = useLocalSearchParams<{ token?: string }>()
  const token = typeof params.token === 'string' ? params.token : ''
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [done, setDone] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async () => {
    setError(null)
    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }
    if (password !== confirm) {
      setError('Passwords do not match')
      return
    }
    setIsLoading(true)
    try {
      await postResetPassword(token, password)
      setDone(true)
    } catch (err) {
      setError(err instanceof AuthApiError || err instanceof Error ? err.message : 'Something went wrong.')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <KeyboardAvoidingView behavior={Platform.OS === 'ios' ? 'padding' : undefined} className="flex-1 bg-spade-bg">
      <ScrollView contentContainerClassName="grow justify-center px-4 py-8">
        <View className="rounded-spade-lg border border-spade-cream/10 bg-[#102316] p-6">
          {!token ? (
            <>
              <Text className="text-2xl font-medium text-spade-cream">Invalid link</Text>
              <Text className="mt-2 text-sm text-spade-gray-2">This reset link is missing its token.</Text>
              <View className="mt-6 items-center">
                <Link href="/(auth)/forgot-password" className="text-sm text-spade-gold">Request a new link</Link>
              </View>
            </>
          ) : done ? (
            <>
              <Text className="text-2xl font-medium text-spade-cream">Password updated</Text>
              <Text className="mt-2 text-sm text-spade-gray-2">Sign in with your new password.</Text>
              <Button className="mt-6" onPress={() => router.replace('/(auth)')}>Go to sign in</Button>
            </>
          ) : (
            <>
              <Text className="text-2xl font-medium text-spade-cream">Choose a new password</Text>
              <View className="mt-6 gap-2">
                <Text className="text-xs font-medium uppercase text-spade-gray-2">New password</Text>
                <TextInput
                  value={password}
                  onChangeText={setPassword}
                  placeholder="At least 8 characters"
                  placeholderTextColor="#9c958966"
                  secureTextEntry
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
                />
              </View>
              <View className="mt-3 gap-2">
                <Text className="text-xs font-medium uppercase text-spade-gray-2">Confirm password</Text>
                <TextInput
                  value={confirm}
                  onChangeText={setConfirm}
                  placeholder="Re-enter your password"
                  placeholderTextColor="#9c958966"
                  secureTextEntry
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
                />
              </View>
              {error ? <Text className="mt-3 text-xs text-spade-red">{error}</Text> : null}
              <Button className="mt-6" onPress={handleSubmit} disabled={isLoading || !password || !confirm}>
                {isLoading ? 'Updating…' : 'Update password'}
              </Button>
            </>
          )}
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  )
}
