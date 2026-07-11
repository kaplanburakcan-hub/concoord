/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        // İPKS kimliği: şantiye betonu + emniyet sarısı vurgusu
        beton: { 950: "#16181d", 900: "#1e2127", 800: "#282c34", 400: "#8b919d", 200: "#d3d6db" },
        emniyet: { 500: "#f5b301", 600: "#d99e00" },
      },
      fontFamily: {
        display: ["'Archivo'", "system-ui", "sans-serif"],
        body: ["'IBM Plex Sans'", "system-ui", "sans-serif"],
        mono: ["'IBM Plex Mono'", "monospace"],
      },
    },
  },
  plugins: [],
};
