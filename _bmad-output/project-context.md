---
project_name: 'canectors-bmad'
user_name: 'Cano'
date: '2026-01-11'
sections_completed: ['technology_stack', 'language_specific', 'framework_specific', 'testing', 'code_quality', 'workflow', 'critical_rules']
status: 'complete'
rule_count: 50+
optimized_for_llm: true
existing_patterns_found: 0
---

# Project Context for AI Agents

_This file contains critical rules and patterns that AI agents must follow when implementing code in this project. Focus on unobvious details that agents might otherwise miss._

---

## Technology Stack & Versions

**Core Stack:**
- Next.js 15 (App Router)
- TypeScript (strict mode)
- Prisma ORM (PostgreSQL - Supabase/Neon)
- tRPC (API type-safe)
- Tailwind CSS
- Clerk (authentication)
- Redis Upstash (cache + queue)

**Runtime CLI (Separate Project):**
- Go (latest stable)

**Key Dependencies:**
- Zod (latest) - validation schemas
- PostgreSQL (managed via Supabase/Neon)

**Version Constraints:**
- Next.js 15+ required (App Router)
- TypeScript strict mode mandatory
- Prisma latest (compatible with PostgreSQL)

## Critical Implementation Rules

### Language-Specific Rules

**TypeScript Configuration:**
- Strict mode mandatory - no `any` types, use `unknown` if needed
- Type safety end-to-end: Zod → tRPC → Prisma
- Shared types in `/types/` or inline, avoid duplication

**Import/Export:**
- Use ESM (`import`/`export`) - no CommonJS `require()` unless necessary
- Type imports: `import type { ... }` for type-only imports

**Error Handling:**
- Always use `TRPCError` for API errors (never `throw new Error()`)
- Log errors with context: `{ organisationId, userId, error }`
- User-facing messages: clear, no stack traces
- Format: `throw new TRPCError({ code: "NOT_FOUND", message: "..." })`

**Async/Await:**
- Prefer `async/await` over Promise chains
- Handle errors with try/catch, not `.catch()`

### Framework-Specific Rules

**React Server Components:**
- Default to Server Components - only add `'use client'` when interactivity required
- Client Components only for: forms, buttons, modals, interactive UI
- Server state via tRPC queries/mutations, local UI state via `useState`

**Next.js App Router:**
- Routes in `/app/{route}/page.tsx`
- Layouts in `/app/{route}/layout.tsx`
- Middleware in `/server/middleware.ts` (extracts `organisationId` from Clerk)

**tRPC Patterns:**
- Routers in `/server/api/routers/{domain}/{router}.router.ts`
- Procedures: `camelCase` with action verb (e.g., `getConnector`, `createPipeline`)
- Always validate input with Zod schemas
- Return data directly (no wrapper): `return connector;` not `return { data: connector };`
- Use `protectedProcedure` for authenticated routes
- Always filter by `organisationId` in database queries: `where: { organisationId: ctx.organisationId }`

**Multi-Tenant Isolation:**
- `organisationId` always in tRPC context (from Clerk middleware)
- Every database query MUST filter by `organisationId`
- Verify resource ownership before mutations

### Testing Rules

**Test Organization:**
- Co-locate tests with source files: `*.test.ts` or `*.spec.ts`
- Same directory structure as source code
- Use Vitest (configured in T3 Stack)

**Test Structure:**
- Unit tests for isolated functions/services
- Integration tests for tRPC routers (test with mock context)
- Component tests with Testing Library (for Client Components)

**Test Patterns:**
- Mock Prisma client for database tests
- Mock tRPC context with `organisationId` for router tests
- Use Zod schemas for test data validation
- Test multi-tenant isolation: verify `organisationId` filtering

**Coverage:**
- Focus on critical paths: auth, multi-tenant isolation, data validation
- Test error cases: `TRPCError` codes, validation failures

### Code Quality & Style Rules

**Naming Conventions:**
- Database: `snake_case` (tables: `users`, columns: `user_id`, `organisation_id`)
- API: `camelCase` (routers: `connectorRouter`, procedures: `getConnector`, params: `connectorId`)
- Components: `PascalCase` (React: `UserCard`, `ConnectorList`)
- Files: `kebab-case.tsx` (components), `camelCase.ts` (utils)
- Constants: `UPPER_SNAKE_CASE` (e.g., `MAX_CONNECTOR_COUNT`)

**Code Organization:**
- Components: `/app/components/{feature}/` (e.g., `/app/components/connectors/`)
- Utils: `/utils/` (e.g., `/utils/format.ts`, `/utils/validation.ts`)
- Services: `/server/services/{domain}/` (e.g., `/server/services/pipeline-generator/`)
- Routers: `/server/api/routers/{domain}/` (e.g., `/server/api/routers/connector/`)

**Library Usage:**
- ✅ ALWAYS check if an existing library/package exists before implementing complex functionality
- ❌ NEVER reinvent the wheel - prefer well-maintained, tested libraries
- Research available options first: npm packages (TypeScript/Node.js), Go modules (runtime CLI)
- ✅ ALWAYS ask the user for approval before implementing/using a library - present the library options with pros/cons
- ❌ NEVER add a library dependency without explicit user approval
- Only implement custom solutions when no suitable library exists or when specific requirements justify it

**Linting/Formatting:**
- ESLint + Prettier pre-configured (T3 Stack)
- Follow T3 Stack conventions
- No trailing commas in JSON (unless required)
- **Go Runtime CLI:** Run `golangci-lint run` after each implementation to verify linting passes

**Documentation:**
- JSDoc comments for complex functions
- Inline comments for unobvious logic
- README updates for new features

### Development Workflow Rules

**Git/Repository:**
- CI/CD: GitHub Actions (automatic deployment to Vercel)
- Database migrations: Prisma Migrate (`prisma migrate dev`)
- Never commit `.env.local` (use `.env.example` as template)

**Environment Configuration:**
- Environment variables: Vercel Dashboard (production) + `.env.local` (development)
- Validation: Zod schemas for env vars (T3 Stack pattern)
- Required vars documented in `.env.example`

**Build & Deployment:**
- Build: `npm run build` (Next.js)
- Prisma: `prisma generate` before build
- Deployment: Automatic via Vercel on push to main
- Runtime CLI: Separate build/deploy (Docker workers on Railway/Fly.io)

**Development Server:**
- Start: `npm run dev` (Next.js dev server)
- Hot reloading: Automatic for `/app/`, `/components/`, `/server/`
- TypeScript: Compilation errors block development

**Code Quality Checks:**
- ✅ ALWAYS run linting checks before considering implementation complete
- **TypeScript/Next.js:** ESLint runs automatically in CI/CD
- **Go Runtime CLI:** Run `golangci-lint run` at the end of each implementation to ensure code quality
- Fix all linting errors before marking task as complete

### Critical Don't-Miss Rules

**Multi-Tenant Isolation (CRITICAL):**
- ❌ NEVER query database without `organisationId` filter
- ✅ ALWAYS: `where: { organisationId: ctx.organisationId, ... }`
- ✅ ALWAYS verify resource ownership before mutations
- ❌ NEVER trust client-provided `organisationId` - use context only

**Error Handling:**
- ❌ NEVER use `throw new Error()` in tRPC procedures
- ✅ ALWAYS use `TRPCError` with appropriate codes
- ❌ NEVER expose stack traces to users
- ✅ ALWAYS log errors with context: `{ organisationId, userId, error }`

**React Components:**
- ❌ NEVER forget `'use client'` on interactive components (forms, buttons)
- ✅ ALWAYS default to Server Components
- ❌ NEVER use `loading` variable name
- ✅ ALWAYS use `isLoading` or `isFetching` from tRPC hooks

**Data Formats:**
- ❌ NEVER use timestamp numbers for dates in JSON
- ✅ ALWAYS use ISO 8601 strings: `YYYY-MM-DDTHH:mm:ss.sssZ`
- ❌ NEVER use `undefined` in JSON responses
- ✅ ALWAYS use `null` explicitly

**API Responses:**
- ❌ NEVER wrap responses: `return { data: connector }`
- ✅ ALWAYS return directly: `return connector;`
- ❌ NEVER use custom status codes in tRPC
- ✅ ALWAYS use `TRPCError` codes: `NOT_FOUND`, `UNAUTHORIZED`, etc.

**Security:**
- ❌ NEVER trust client input - always validate with Zod
- ❌ NEVER expose sensitive data in error messages
- ✅ ALWAYS encrypt credentials before storing (use `/utils/encryption.ts`)
- ✅ ALWAYS validate `organisationId` in every database operation

---

## Usage Guidelines

**For AI Agents:**

- Read this file before implementing any code
- Follow ALL rules exactly as documented
- When in doubt, prefer the more restrictive option
- Update this file if new patterns emerge

**For Humans:**

- Keep this file lean and focused on agent needs
- Update when technology stack changes
- Review quarterly for outdated rules
- Remove rules that become obvious over time

**Last Updated:** 2026-01-11
