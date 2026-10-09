import { fileURLToPath } from 'node:url';
import js from '@eslint/js';
import globals from 'globals';

export default [
  {
    ...js.configs.recommended,
    basePath: fileURLToPath(new URL('..', import.meta.url)),
    files: ['scripts/**/*.mjs'],
    languageOptions: { globals: globals.node },
  },
];
