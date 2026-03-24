import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
  ]),
  {
    rules: {
      // Reading localStorage synchronously in useEffect is the standard
      // Next.js pattern for client-side auth checks (localStorage is not
      // available during SSR so the read cannot happen at render time).
      "react-hooks/set-state-in-effect": "off",
    },
  },
]);

export default eslintConfig;
