// ESLint (flat config) — CI kalite kapısı.
//
// Kural seçimi bilinçli olarak dar: gerçek hata üreten kalıplar (hook kuralları,
// tanımsız değişken, erişilemeyen kod vb.) `error`, stil/temizlik önerileri
// `warn`. Mevcut kod tabanındaki uyarılar CI'ı kırmaz; yeni hatalar kırar.
import js from "@eslint/js";
import globals from "globals";
import reactHooks from "eslint-plugin-react-hooks";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["dist", "node_modules", "public"] },
  {
    files: ["**/*.{ts,tsx}"],
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    languageOptions: {
      ecmaVersion: 2021,
      globals: { ...globals.browser },
    },
    plugins: { "react-hooks": reactHooks },
    rules: {
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
      "@typescript-eslint/no-explicit-any": "warn",
      "@typescript-eslint/no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_", caughtErrors: "none" },
      ],
      "no-empty": ["warn", { allowEmptyCatch: true }],
    },
  },
);
