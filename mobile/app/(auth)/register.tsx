import { useState } from 'react'
import { KeyboardAvoidingView, Platform, Pressable, ScrollView, Text, TextInput, View } from 'react-native'
import { Link } from 'expo-router'
import { Button } from '../../src/components/Button'
import { postRegister, AuthApiError } from '../../src/api/auth'
import { useAuth } from '../../src/hooks/useAuth'

// Native port of web/src/pages/RegisterPage.tsx. Same client-side validation;
// on success it stores the session and the root auth gate routes to the lobby.
export default function RegisterScreen() {
  const { login } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [username, setUsername] = useState('')
  const [termsAccepted, setTermsAccepted] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const usernameValid = /^[a-z0-9_]{3,32}$/.test(username)
  const isSubmitDisabled =
    isLoading || !email || !password || !confirmPassword || !displayName.trim() || !usernameValid || !termsAccepted

  const handleSubmit = async () => {
    setError(null)
    if (!termsAccepted) {
      setError('You must accept the terms to create an account')
      return
    }
    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }
    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }
    if (!displayName.trim() || displayName.length > 50) {
      setError('Display name must be 1-50 characters')
      return
    }
    if (!usernameValid) {
      setError('Username must be 3-32 characters and use lowercase letters, numbers, or underscores')
      return
    }

    setIsLoading(true)
    try {
      const response = await postRegister(email, password, displayName, username)
      login(response.jwt, response.refreshToken)
    } catch (err) {
      if (err instanceof AuthApiError || err instanceof Error) {
        setError(err.message)
      } else {
        setError('An unexpected error occurred')
      }
    } finally {
      setIsLoading(false)
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
            <Text className="text-2xl font-medium text-spade-cream">Create Account</Text>
            <Text className="mt-1.5 text-sm text-spade-gray-2">Enter your details to join the club.</Text>
          </View>

          <View className="gap-4">
            <Field label="Display name">
              <TextInput
                value={displayName}
                onChangeText={setDisplayName}
                placeholder="TableMaster99"
                placeholderTextColor="#9c958966"
                maxLength={50}
                editable={!isLoading}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
              />
            </Field>

            <Field label="Username" hint="Friends add you by @username. Lowercase letters, numbers, underscores, 3-32 chars.">
              <TextInput
                value={username}
                onChangeText={(v) => setUsername(v.toLowerCase())}
                placeholder="table_master_99"
                placeholderTextColor="#9c958966"
                maxLength={32}
                autoCapitalize="none"
                autoCorrect={false}
                editable={!isLoading}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
              />
            </Field>

            <Field label="Email">
              <TextInput
                value={email}
                onChangeText={setEmail}
                placeholder="you@example.com"
                placeholderTextColor="#9c958966"
                autoCapitalize="none"
                keyboardType="email-address"
                editable={!isLoading}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
              />
            </Field>

            <Field label="Password">
              <TextInput
                value={password}
                onChangeText={setPassword}
                placeholder="Minimum 8 characters"
                placeholderTextColor="#9c958966"
                secureTextEntry
                editable={!isLoading}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
              />
            </Field>

            <Field label="Confirm password">
              <TextInput
                value={confirmPassword}
                onChangeText={setConfirmPassword}
                placeholder="Re-enter password"
                placeholderTextColor="#9c958966"
                secureTextEntry
                editable={!isLoading}
                className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
              />
            </Field>

            <Pressable className="flex-row items-start gap-3" onPress={() => setTermsAccepted((v) => !v)}>
              <View className={`mt-0.5 size-5 items-center justify-center rounded-spade-sm border ${termsAccepted ? 'border-spade-gold bg-spade-gold' : 'border-spade-gray-4'}`}>
                {termsAccepted ? <Text className="text-xs text-[#1a0e00]">✓</Text> : null}
              </View>
              <Text className="flex-1 text-xs leading-5 text-spade-gray-2">
                I agree to the Terms of Service and Privacy Policy.
              </Text>
            </Pressable>

            {error ? (
              <View className="rounded-spade-md border border-spade-red/50 bg-spade-red/10 px-3 py-2">
                <Text className="text-sm text-[#ffb4ab]">{error}</Text>
              </View>
            ) : null}

            <Button onPress={handleSubmit} disabled={isSubmitDisabled}>
              {isLoading ? 'Creating account...' : 'Create Account'}
            </Button>
          </View>

          <View className="mt-6 flex-row justify-center">
            <Text className="text-sm text-spade-gray-3">Already have an account? </Text>
            <Link href="/(auth)" className="text-sm font-medium text-spade-gold">Sign In</Link>
          </View>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  )
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <View className="gap-1.5">
      <Text className="text-xs font-medium uppercase text-spade-gray-2">{label}</Text>
      {children}
      {hint ? <Text className="text-[11px] text-spade-gray-3">{hint}</Text> : null}
    </View>
  )
}
