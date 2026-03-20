---
applyTo: "**/*.ts,**/*.tsx,**/*.js"
description: "TypeScript/JavaScript conventions for the az-prom-rules-converter tool."
---

# TypeScript Conventions

- Use ES6 `import` statements — no CommonJS `require()`.
- Annotate function parameters and return types with TypeScript types.
- Use `async/await` for asynchronous operations — avoid raw Promise chains.
- Use type-safe result objects (`StepResult`) for error handling instead of `throw/catch`.
- Use Commander.js for CLI argument parsing.
- Format JSON output with `JSON.stringify(obj, null, 2)`.
- Write tests using Jest with `describe`/`test`/`expect` patterns.
- Place test files alongside source with `.test.ts` suffix.
- Build with `npm run build` (runs `tsc`), test with `npm test` (runs `jest`).
