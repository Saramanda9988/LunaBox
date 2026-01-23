import antfu from "@antfu/eslint-config";
import pluginQuery from "@tanstack/eslint-plugin-query";
import reactHooks from "eslint-plugin-react-hooks";

export default antfu(
  {
    type: "app",
    react: true,
    typescript: true,
    formatters: true,
    stylistic: {
      indent: 2,
      semi: true,
      quotes: "double",
    },
    ignores: [
      "public/**",
      "api/**",
      ".github/**",
      "wailsjs/**",
      "src/vite-env.d.ts",
      "**/*.md",
      "bindings/**",
    ],
    plugins: {
      "@tanstack/query": pluginQuery,
      "react-hooks": reactHooks,
    },
  },
  {
    files: ["**/*.js", "**/*.ts"],
    rules: {
      "@tanstack/query/exhaustive-deps": "error",
      "ts/no-redeclare": "off",
      "ts/consistent-type-definitions": ["error", "type"],
      "no-console": ["error", { allow: ["warn", "error"] }], // 生产环境不应该有 console.log，但 warn/error 允许在开发时使用
      "antfu/no-top-level-await": ["off"],
      "node/prefer-global/process": ["off"],
      "style/multiline-ternary": ["off"],
      "perfectionist/sort-imports": [
        "error",
        {
          tsconfigRootDir: ".",
        },
      ],
      "unicorn/filename-case": [
        "error",
        {
          cases: {
            camelCase: true,
            pascalCase: true,
          },
          ignore: ["README.md"],
        },
      ],
    },
  },
  {
    files: ["**/*.{js,jsx,ts,tsx}"],
    rules: {
      // Core React Hooks rules
      "react-hooks/rules-of-hooks": "error",
      "react-hooks/exhaustive-deps": "warn",
      "style/multiline-ternary": "off",
    },
  },
  {
    files: ["api/**/*.js", "api/**/*.ts", "services/**/*.ts"],
    rules: {
      "eslint-comments/no-unlimited-disable": "off",
    },
  },
  {
    files: ["*.config.ts"],
    rules: {
      "unicorn/filename-case": ["off"],
    },
  },
);
