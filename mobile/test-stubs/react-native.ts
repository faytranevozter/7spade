// Minimal react-native stub for pure-logic unit tests. Only the APIs touched at
// import time by the modules under test are provided. UI/component tests should
// use the full jest-expo runtime instead.
export const AppState = {
  addEventListener: () => ({ remove: () => {} }),
  currentState: 'active',
}

export const Platform = { OS: 'ios', select: (obj: Record<string, unknown>) => obj.ios }

export const Vibration = { vibrate: () => {}, cancel: () => {} }
