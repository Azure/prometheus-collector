---
applyTo: "**/*.ts,**/*.tsx,**/*.js"
description: TypeScript/JavaScript code style and best practices for this repository.
---

# TypeScript Code Conventions

1. **Strict mode**: TypeScript strict mode is enabled (`"strict": true` in `tsconfig.json`). All code must pass strict type checking.
2. **Naming**: camelCase for variables and functions, PascalCase for types, interfaces, and classes.
3. **Error handling**: Use result objects with `{success: boolean, error?: {title: string, details: string[]}}` pattern rather than throwing exceptions for expected failures.
4. **Testing**: Use Jest with `ts-jest` preset. Test files are `*.test.ts`. Run with `npm test`.
5. **Imports**: Use ES module imports. Group standard/external imports before local imports.
6. **Build**: Compile with `tsc` via `npm run build`. Output goes to `dist/`.
7. **CLI pattern**: Use Commander.js for CLI argument parsing (see `tools/az-prom-rules-converter/`).
