/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        primary: '#2563eb',
        'primary-light': '#3b82f6',
        'primary-dark': '#1d4ed8',
        secondary: '#64748b',
        success: '#10b981',
        warning: '#f59e0b',
        danger: '#ef4444',
        'dark-100': '#334155',
        'dark-200': '#1e293b',
        'dark-300': '#0f172a',
        'light-100': '#f8fafc',
        'light-200': '#e2e8f0',
        'light-300': '#cbd5e1',
      },
      fontFamily: {
        inter: ['Inter', 'sans-serif'],
        mono: ['SF Mono', 'Monaco', 'Menlo', 'Consolas', 'monospace'],
      },
      boxShadow: {
        'sm': '0 1px 3px rgba(0, 0, 0, 0.1)',
        'md': '0 4px 6px rgba(0, 0, 0, 0.1)',
        'lg': '0 10px 15px rgba(0, 0, 0, 0.1)',
        'dark-sm': '0 1px 3px rgba(0, 0, 0, 0.3)',
        'dark-md': '0 4px 6px rgba(0, 0, 0, 0.3)',
        'dark-lg': '0 10px 15px rgba(0, 0, 0, 0.3)',
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
      }
    },
  },
  plugins: [],
}

