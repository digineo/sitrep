import js from "@eslint/js"
import stylistic from "@stylistic/eslint-plugin"
import { defineConfig } from "eslint/config"
import simpleImportSort from "eslint-plugin-simple-import-sort"
import vue from "eslint-plugin-vue"
import tseslint from "typescript-eslint"

export default defineConfig(
  { ignores: ["dist/", "playwright-report/", "test-results/", "e2e/.tmp/"] },

  js.configs.recommended,
  tseslint.configs.recommended,
  vue.configs["flat/recommended"],

  {
    files:           ["**/*.vue"],
    languageOptions: { parserOptions: { parser: tseslint.parser } },
  },

  stylistic.configs.customize({
    indent:      2,
    quotes:      "double",
    semi:        false,
    braceStyle:  "1tbs",
    commaDangle: "always-multiline",
    jsx:         false,
  }),

  {
    plugins: { "simple-import-sort": simpleImportSort },

    rules: {
      // Where the house style differs from what `customize` sets.
      "@stylistic/array-bracket-newline":          ["error", "consistent"],
      "@stylistic/array-element-newline":          ["error", "consistent"],
      "@stylistic/brace-style":                    ["error", "1tbs", { allowSingleLine: false }],
      "@stylistic/function-call-argument-newline": ["error", "consistent"],
      "@stylistic/function-paren-newline":         ["error", "multiline-arguments"],
      "@stylistic/indent":                         ["error", 2, {
        VariableDeclarator: {
          var:   2,
          let:   2,
          const: 3,
        },
      }],
      "@stylistic/key-spacing":     ["error", { align: "value" }],
      // `align: "value"` aligns type members too, and the rule's stock
      // exception covers object properties only; without this the two fight
      // over the same lines and ESLint reports a circular fix.
      "@stylistic/no-multi-spaces": ["error", {
        exceptions: {
          Property:         true,
          TSTypeAnnotation: true,
        },
      }],
      // One entry per line once an object literal has two of them.
      "@stylistic/object-curly-newline": ["error", {
        ObjectExpression: {
          multiline:     true,
          minProperties: 2,
        },
        ObjectPattern: {
          multiline:  true,
          consistent: true,
        },
        ImportDeclaration: {
          multiline:  true,
          consistent: true,
        },
        ExportDeclaration: {
          multiline:  true,
          consistent: true,
        },
        TSTypeLiteral: {
          multiline:  true,
          consistent: true,
        },
        TSInterfaceBody: {
          multiline:  true,
          consistent: true,
        },
      }],
      "@stylistic/object-property-newline":         ["error"],
      "@stylistic/padding-line-between-statements": ["error",
        {
          blankLine: "always",
          prev:      "block-like",
          next:      "*",
        },
        {
          blankLine: "never",
          prev:      "block-like",
          next:      ["break", "return"],
        },
        {
          blankLine: "never",
          prev:      ["if", "switch", "for"],
          next:      ["if", "switch", "for"],
        },
        {
          blankLine: "always",
          prev:      ["break", "return"],
          next:      "case",
        },
        {
          blankLine: "always",
          prev:      "function",
          next:      "function",
        }],
      "@stylistic/quotes": ["error", "double", {
        avoidEscape:           true,
        allowTemplateLiterals: "avoidEscape",
      }],
      "@stylistic/space-before-function-paren": ["error", "never"],
      "@stylistic/space-infix-ops":             ["error", { int32Hint: true }],
      "@stylistic/spaced-comment":              ["error", "always", { markers: ["=", "#region", "#endregion", "/"] }],

      "curly": ["error", "all"],

      // vue-tsc checks identifiers
      "no-undef": "off",

      "simple-import-sort/imports": "error",
      "simple-import-sort/exports": "error",

      // Vue.
      "vue/block-order": ["error", {
        order: [
          "script[lang='ts']:not([setup])",
          "script[setup]",
          "template",
          "style:not([scoped])",
          "style[scoped]",
        ],
      }],
      "vue/block-spacing":                     ["error", "always"],
      "vue/block-tag-newline":                 ["error", { maxEmptyLines: 0 }],
      "vue/component-name-in-template-casing": ["error", "PascalCase", { registeredComponentsOnly: false }],
      "vue/first-attribute-linebreak":         "error",
      "vue/html-indent":                       ["error", 2, {
        attribute:                 2,
        alignAttributesVertically: false,
      }],
      "vue/html-self-closing":       ["error", { html: { component: "always" } }],
      "vue/max-attributes-per-line": ["error", {
        singleline: { max: 2 },
        multiline:  { max: 1 },
      }],
      "vue/no-v-html":                   "error",
      "vue/padding-line-between-blocks": ["error", "always"],
    },
  },

  {
    // Test tables keep one case per line, objects in it included.
    files: ["**/*.test.ts", "e2e/**/*.spec.ts"],
    rules: {
      "@stylistic/object-curly-newline": ["error", {
        multiline:  true,
        consistent: true,
      }],
      "@stylistic/object-property-newline": ["error", { allowAllPropertiesOnSameLine: true }],
    },
  },
)
