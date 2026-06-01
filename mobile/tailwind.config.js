/** @type {import('tailwindcss').Config} */
// NativeWind v4 config. The `spade-*` design tokens mirror web/src/index.css's
// @theme block so the mobile UI matches the web app's palette and radii.
module.exports = {
  content: ['./app/**/*.{js,jsx,ts,tsx}', './src/**/*.{js,jsx,ts,tsx}'],
  presets: [require('nativewind/preset')],
  theme: {
    extend: {
      colors: {
        'spade-bg': '#0d1a12',
        'spade-green': '#1a472a',
        'spade-green-mid': '#235c36',
        'spade-green-light': '#2d7a46',
        'spade-gold': '#c9922b',
        'spade-gold-light': '#f5c842',
        'spade-cream': '#f4ead5',
        'spade-red': '#c0392b',
        'spade-red-dark': '#922b21',
        'spade-black': '#1a1a1a',
        'spade-white': '#fafaf8',
        'spade-gray-1': '#f0ece3',
        'spade-gray-2': '#d9d4c8',
        'spade-gray-3': '#9c9589',
        'spade-gray-4': '#5a5550',
        'spade-blue-suit': '#1e4080',
      },
      borderRadius: {
        'spade-sm': '4px',
        'spade-md': '8px',
        'spade-lg': '12px',
        'spade-card': '10px',
        'spade-xl': '18px',
        'spade-pill': '20px',
      },
      fontFamily: {
        sans: ['DMSans', 'system-ui', 'sans-serif'],
        mono: ['DMMono', 'monospace'],
      },
    },
  },
  plugins: [],
}
