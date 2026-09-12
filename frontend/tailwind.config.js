/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        background: 'var(--color-bg)',
        surface: 'var(--color-surface)',
        surfaceHover: 'var(--color-surface-hover)',
        border: 'var(--color-border)',
        foreground: 'var(--color-text)',
        muted: 'var(--color-text-muted)',
        primary: {
          DEFAULT: 'var(--color-primary, #ef4444)',
          hover: 'var(--color-primary-hover, #dc2626)',
        },
        accent: 'var(--color-accent, #3b82f6)',
      },
    },
  },
  plugins: [],
};
