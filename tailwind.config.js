/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './internal/views/templates/**/*.html',
    './internal/handlers/**/*.go',
    './cmd/**/*.go',
    './main.go',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#f5f3ff',
          100: '#ede9fe',
          200: '#ddd6fe',
          300: '#c4b5fd',
          400: '#a78bfa',
          500: '#8b5cf6',
          600: '#7c3aed',
          700: '#6d28d9',
          800: '#5b21b6',
          900: '#4c1d95',
        },
        dark: {
          bg: '#0f172a',
          surface: '#1e293b',
          card: '#1e293b',
          border: '#334155',
        }
      },
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography'),
    require('@tailwindcss/container-queries'),
  ],
}
