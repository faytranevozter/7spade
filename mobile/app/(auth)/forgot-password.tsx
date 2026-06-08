import { useState } from 'react'
import { KeyboardAvoidingView, Platform, ScrollView, Text, TextInput, View } from 'react-native'
import { Link } from 'expo-router'
import { Button } from '../../src/components/Button'
import { postForgotPassword, AuthApiError } from '../../src/api/auth'

// Native port of web ForgotPasswordPage. Requests a reset email; always shows a
// neutral confirmation so it can't be used to probe which emails are registered.
export default function ForgotPasswordScreen() {
  const [email, setEmail] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [submitted, setSubmitted] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async () => {
    setError(null)
    setIsLoading(true)
    try {
      await postForgotPassword(email.trim().toLowerCase())
      setSubmitted(true)
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
          {submitted ? (
            <>
              <Text className="text-2xl font-medium text-spade-cream">Check your inbox</Text>
              <Text className="mt-2 text-sm text-spade-gray-2">
                If an account exists for that email, we've sent a reset link. It expires in 15 minutes.
              </Text>
            </>
          ) : (
            <>
              <Text className="text-2xl font-medium text-spade-cream">Reset your password</Text>
              <Text className="mt-2 text-sm text-spade-gray-2">Enter your email and we'll send a reset link.</Text>
              <View className="mt-6 gap-2">
                <Text className="text-xs font-medium uppercase text-spade-gray-2">Email</Text>
                <TextInput
                  value={email}
                  onChangeText={setEmail}
                  placeholder="you@example.com"
                  placeholderTextColor="#9c958966"
                  autoCapitalize="none"
                  keyboardType="email-address"
                  className="rounded-spade-md border border-spade-gray-4/60 bg-spade-bg px-3 py-3 text-sm text-spade-cream"
                />
              </View>
              {error ? <Text className="mt-3 text-xs text-spade-red">{error}</Text> : null}
              <Button className="mt-6" onPress={handleSubmit} disabled={isLoading || !email}>
                {isLoading ? 'Sending…' : 'Send reset link'}
              </Button>
            </>
          )}
          <View className="mt-6 items-center">
            <Link href="/(auth)" className="text-sm text-spade-gold">Back to sign in</Link>
          </View>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  )
}
