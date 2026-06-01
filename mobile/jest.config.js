// Pure-logic unit tests run under ts-jest in a node environment. The modules
// under test (cards, claims, the useGameSocket helpers) only need a handful of
// native APIs at import time, which we stub via moduleNameMapper so we don't
// have to boot the full jest-expo/React Native runtime for logic tests.
module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  testMatch: ['**/__tests__/**/*.test.ts'],
  moduleNameMapper: {
    '^react-native$': '<rootDir>/test-stubs/react-native.ts',
    '^expo-constants$': '<rootDir>/test-stubs/expo-constants.ts',
  },
  transform: {
    '^.+\\.tsx?$': [
      'ts-jest',
      {
        // Transpile-only: skip type diagnostics here (the full typecheck runs
        // separately via `tsc --noEmit`). This keeps logic tests fast and avoids
        // resolving the full RN/React type graph in the test runner.
        isolatedModules: true,
        diagnostics: false,
        tsconfig: { jsx: 'react-jsx', esModuleInterop: true },
      },
    ],
  },
}
