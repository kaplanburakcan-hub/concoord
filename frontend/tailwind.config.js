/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: ["selector", '[data-theme="dark"]'],
  theme: {
    extend: {
      colors: {
        // Token'lar CSS değişkenlerine bağlı (index.css) — tüm sayfalar tek
        // seferde yeni palete ve light/dark'a geçer. RGB kanal biçimi sayesinde
        // opaklık modifikatörleri (ör. bg-emniyet-500/15) çalışmaya devam eder.
        beton: {
          950: "rgb(var(--beton-950) / <alpha-value>)",
          900: "rgb(var(--beton-900) / <alpha-value>)",
          800: "rgb(var(--beton-800) / <alpha-value>)",
          700: "rgb(var(--beton-700) / <alpha-value>)",
          600: "rgb(var(--beton-600) / <alpha-value>)",
          500: "rgb(var(--beton-500) / <alpha-value>)",
          400: "rgb(var(--beton-400) / <alpha-value>)",
          300: "rgb(var(--beton-300) / <alpha-value>)",
          200: "rgb(var(--beton-200) / <alpha-value>)",
          100: "rgb(var(--beton-100) / <alpha-value>)",
        },
        emniyet: {
          500: "rgb(var(--emniyet-500) / <alpha-value>)",
          600: "rgb(var(--emniyet-600) / <alpha-value>)",
        },
        sky2: {
          500: "rgb(var(--sky) / <alpha-value>)",
        },
      },
      fontFamily: {
        // Tüm roller Aptos Light (Windows/Office'te yerleşik); yoksa Inter'e düşer.
        display: ["'Aptos'", "'Aptos Display'", "'Segoe UI Variable'", "'Segoe UI'", "'Inter'", "system-ui", "sans-serif"],
        body: ["'Aptos'", "'Segoe UI Variable'", "'Segoe UI'", "'Inter'", "system-ui", "sans-serif"],
        mono: ["'Aptos'", "'Segoe UI Variable'", "'Inter'", "system-ui", "sans-serif"],
      },
    },
  },
  plugins: [],
};
