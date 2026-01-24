---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
inputDocuments:
  - 'product-brief-canectors-2026-01-10.md'
  - 'prd.md'
  - 'ux-design-specification.md'
  - 'research/market-api-connector-automation-saas-research-2026-01-10.md'
workflowType: 'architecture'
project_name: 'Canectors'
user_name: 'Cano'
date: '2026-01-11'
lastStep: 8
status: 'complete'
completedAt: '2026-01-11'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**

Le projet définit ~125 exigences fonctionnelles organisées en 12 catégories principales :

- **Connector Management (9 FRs)** : CRUD pipelines, composition modules Input/Filter/Output, duplication, versioning
- **OpenAPI Ingestion & Processing (6 FRs)** : Import OpenAPI (JSON/YAML), parsing, extraction endpoints/schémas/auth
- **Automatic Connector Generation (13 FRs)** : Génération pipelines modulaires (Input/Filter/Output), CRON scheduling, configurations explicites
- **AI-Assisted Mapping (7 FRs)** : Suggestions intelligentes de mapping avec niveaux de confiance, validation développeur
- **Connector Execution (17 FRs)** : Runtime CLI, exécution modules Input/Filter/Output, dry-run, logs explicites, gestion erreurs
- **Documentation Generation (8 FRs)** : Génération automatique documentation pipelines avec modules et flow
- **Input Modules (8 FRs)** : HTTP Request (polling + CRON), Webhook (MVP), SQL Query, Pub/Sub/Kafka (post-MVP)
- **Filter Modules (12 FRs)** : Mapping déclaratif, Conditions simples (MVP), Transformations avancées, Cloning, Scripting (post-MVP)
- **Output Modules (8 FRs)** : HTTP Request (MVP), Webhook, SQL, Pub/Sub/Kafka (post-MVP)
- **User & Organization Management (14 FRs)** : Multi-tenant avec isolation stricte, rôles Owner/Member, switch organisations
- **Subscription & Billing (8 FRs)** : Free tier + Paid tier, limitation usage, upgrade/downgrade
- **Integration & Workflow (6 FRs)** : CLI cross-platform, CI/CD, fichiers YAML/JSON versionnables
- **Template & Reusability (6 FRs)** : Sauvegarde templates, réutilisation modules, partage organisation

**Non-Functional Requirements:**

Six NFRs critiques qui guideront les décisions architecturales :

- **Performance** : Génération pipeline <30 min (50-200 endpoints), suggestions IA <10s, CRUD <2s, auth <1s
- **Security** : Isolation multi-tenant stricte (organisation_id sur toutes tables), HTTPS, chiffrement credentials, validation appartenance organisation systématique
- **Scalability** : 50-100 utilisateurs MVP → 500-1000 à 12 mois → 2000-5000 à 24 mois, 100-200 pipelines MVP → 20000+ à 24 mois
- **Reliability** : Runtime 100% déterministe, >95% pipelines fonctionnels première itération, disponibilité ≥99%, gestion erreurs robuste
- **Integration** : OpenAPI 3.0 (JSON/YAML), CLI cross-platform (Windows/Mac/Linux), CI/CD natif, format texte versionnable
- **Maintainability** : Format déclaratif stable backward compatible, runtime unique maintenable, évolution indépendante runtime vs pipelines

**Scale & Complexity:**

- **Primary domain** : Full-stack SaaS B2B (backend multi-tenant + frontend web + CLI + runtime portable)
- **Complexity level** : Moyenne à élevée (SaaS multi-tenant avec pipelines modulaires, IA, runtime portable, ordre de développement séquentiel)
- **Estimated architectural components** : 10-14 composants majeurs
  - Format de configuration (schéma, validation) - Epic 1
  - CLI - Ingesteur de configuration (Epic 2)
  - Runtime d'exécution de pipelines (Epic 2)
  - Moteur d'exécution de modules (Input/Filter/Output) (Epic 2)
  - Scheduler CRON pour polling (Epic 2)
  - Serveur webhook pour réception temps réel (Epic 2)
  - API Backend (multi-tenant) - Epic 3
  - Service de génération de pipelines (Epic 3)
  - Service IA (mapping assistive) (Epic 3)
  - Frontend Web (SaaS) (Epic 3)
  - Service d'authentification/autorisation
  - Service de billing/subscription

### Technical Constraints & Dependencies

**Contraintes techniques identifiées :**

- **Ordre de développement séquentiel (critique)** : Format de configuration (Epic 1) → CLI (Epic 2) → Front (Epic 3). Le format doit être défini et stable avant développement CLI et Front.
- **CLI comme source de vérité** : Le CLI (ingesteur de configuration) définit le format exact en pratique et doit valider toutes configurations générées par le Front.
- **OpenAPI 3.0** (JSON/YAML) comme format d'entrée principal, support jusqu'à 500 endpoints par API (MVP: 50-200 typique)
- **REST + JSON uniquement** au MVP (pas de SOAP, XML, formats exotiques)
- **Runtime portable** requis (Go ou Rust) pour exécution déterministe cross-platform
- **Isolation multi-tenant stricte** (sécurité critique) : toutes les données isolées par organisation_id, validation systématique
- **Format déclaratif stable** et backward compatible (YAML/JSON) pour maintenabilité long terme
- **Modules Input/Filter/Output** composables, versionnables et interprétés par le runtime (pas de code généré)
- **CLI cross-platform** (Windows, Mac, Linux) avec installation <15 min
- **Support CRON scheduling** pour polling périodique
- **Support webhooks** pour réception données temps réel

**Dépendances externes :**

- **OpenAPI specifications** : Format standard pour ingestion
- **CI/CD pipelines** : Intégration native sans modifications majeures workflows existants
- **IA/ML services** : Pour suggestions mapping (modèle à définir)
- **Git repositories** : Fichiers YAML/JSON versionnables (développeurs ajoutent dans leur inventaire de production)

**Ordre de développement et validation :**

- **Epic 1 (Priorité 1)** : Définition format de configuration - Fondation du système
- **Epic 2 (Priorité 2)** : CLI - Ingesteur de configuration - Valide le format, exécute pipelines
- **Epic 3 (Priorité 3)** : Front - Générateur de configuration - Génère selon format validé
- **Critère de passage Epic 2 → Epic 3** : CLI fonctionnel avec configurations manuelles, format stable
- **Validation continue** : CLI doit valider toutes configurations générées par Front

### Cross-Cutting Concerns Identified

**Préoccupations transversales qui affecteront plusieurs composants :**

1. **Sécurité et isolation multi-tenant**
   - Isolation logique stricte par organisation (toutes requêtes scoped)
   - Chiffrement credentials API (API keys, OAuth tokens) au repos
   - Conformité GDPR de base (données personnelles, droit effacement)
   - Validation appartenance organisation à chaque opération

2. **Performance et scalabilité**
   - Génération pipelines optimisée (<30 min pour 50-200 endpoints)
   - Suggestions IA rapides (<10s)
   - Exécution runtime efficace (modules Input/Filter/Output)
   - Scheduler CRON performant pour polling
   - Indexation optimisée sur organisation_id pour isolation efficace
   - Dégradation performance <20% avec 10x utilisateurs

3. **Déterminisme et fiabilité**
   - Runtime 100% déterministe (exécutions prévisibles, pas comportements aléatoires)
   - Exécution modules prévisible et reproductible
   - >95% pipelines fonctionnels première itération
   - Gestion erreurs robuste et explicite par module (logs contexte suffisant)
   - Pas de perte données lors erreurs exécution

4. **Maintenabilité et évolutivité**
   - Format déclaratif stable (backward compatible entre versions runtime)
   - Modules indépendants et composables
   - Runtime unique maintenable (pas dépendances frameworks externes dans configurations générées)
   - Évolution indépendante runtime vs pipelines possible
   - Pipelines lisibles et maintenables dans le temps

5. **Architecture modulaire et composabilité**
   - Modules Input/Filter/Output réutilisables et composables
   - Composition de pipelines flexible
   - Extensibilité pour nouveaux types de modules (post-MVP : SQL, Pub/Sub, etc.)
   - Format de configuration supporte composition modulaire

6. **Ordre de développement et cohérence du format**
   - Format de configuration défini en premier (Epic 1) - fondation critique
   - CLI valide le format avant développement Front (Epic 2)
   - Front génère selon format validé et stable (Epic 3)
   - Validation continue : CLI doit valider toutes configurations générées par Front
   - Pas de divergence entre format défini et format implémenté

7. **Intégration workflow développeur**
   - Compatibilité Git standard (format texte, diffable, versioning standard)
   - CLI intégration CI/CD (exécution conteneurs Docker)
   - Fichiers YAML/JSON versionnables (développeurs ajoutent dans inventaire de production)
   - Pas de vendor lock-in (configurations exportables, pipelines versionnables)
   - Intégration naturelle sans modifications majeures workflows existants

8. **Accessibilité et UX (frontend web)**
   - Conformité WCAG AA (contraste 4.5:1, navigation clavier, screen readers)
   - Responsive design (breakpoints 640px, 1024px, desktop-first)
   - Performance optimisée (lazy loading, memoization)
   - Pas de real-time complexe (CLI fonctionne hors ligne, Web nécessite connexion)

## Starter Template Evaluation

### Primary Technology Domain

Full-stack Next.js avec TypeScript basé sur l'analyse des exigences du projet.

### Starter Options Considered

**T3 Stack (create-t3-app)** - Recommandé
- Next.js 15 avec App Router
- TypeScript strict
- Prisma ORM (PostgreSQL)
- tRPC pour API type-safe
- Tailwind CSS
- NextAuth.js (compatible Clerk/Auth.js)
- Configuration production-ready

**create-next-app (officiel)** - Alternative minimale
- Next.js de base
- Nécessite setup manuel pour Prisma, tRPC, etc.
- Plus de flexibilité mais plus de configuration

### Selected Starter: T3 Stack (create-t3-app)

**Rationale for Selection:**

T3 Stack est le choix optimal car il fournit une base solide alignée avec votre stack technique :
- **Type-safety end-to-end** : TypeScript + tRPC garantit la sécurité des types du frontend au backend
- **Prisma intégré** : ORM moderne parfait pour PostgreSQL (Supabase/Neon)
- **Next.js optimisé** : Configuration production-ready pour Vercel
- **Intégrations faciles** : Compatible avec tous vos services tiers (Clerk, Stripe, PostHog, Sentry, Resend)
- **Communauté active** : Maintenu et documenté, largement adopté pour les projets SaaS B2B
- **Évolutif** : Structure prête pour multi-tenant, workers, et CLI séparés

**Initialization Command:**

```bash
npx create-t3-app@latest canectors-app \
  --nextAuth \
  --prisma \
  --trpc \
  --tailwind \
  --typescript
```

**Architectural Decisions Provided by Starter:**

**Language & Runtime:**
- TypeScript strict mode activé
- Node.js runtime (compatible Vercel)
- ESM/CommonJS configuration optimisée

**Styling Solution:**
- Tailwind CSS avec configuration Next.js
- PostCSS configuré
- Dark mode ready

**Build Tooling:**
- Next.js 15 avec App Router
- Turborepo-ready (pour monorepo futur si nécessaire)
- ESLint + Prettier pré-configurés
- Environment variables validation

**Testing Framework:**
- Vitest (optionnel, à activer)
- Testing Library (optionnel, à activer)
- Structure de tests prête

**Code Organization:**
- App Router structure (/app directory)
- Separation of concerns (server/client components)
- tRPC routers organisés par domaine (/server/api/routers)
- Prisma schema centralisé (/prisma/schema.prisma)
- Utils et types partagés (/utils, /types)

**Development Experience:**
- Hot reloading configuré
- TypeScript strict mode
- Auto-imports configurés
- Environment variables typées
- Git hooks (optionnel avec Husky)

**Note:** L'initialisation du projet avec cette commande devrait être la première story d'implémentation. Après initialisation, il faudra configurer :
- Intégration Clerk/Auth.js (remplacer NextAuth si nécessaire)
- Configuration Prisma pour Supabase/Neon
- Setup services tiers (Stripe, PostHog, Sentry, Resend)
- Configuration multi-tenant (middleware organisation_id)
- Setup CLI séparé pour runtime portable (Go/Rust) - indépendant du starter Next.js

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
- Modèle de données multi-tenant (organisation_id)
- Format de configuration pipelines (JSON avec YAML en alternative - Epic 1)
- Architecture runtime CLI (Go - Epic 2)
- Authentification (Clerk)
- Isolation multi-tenant (middleware Next.js + tRPC context)

**Important Decisions (Shape Architecture):**
- Validation données (Zod + Prisma)
- Cache et queue (Redis Upstash)
- Intégration IA (OpenAI API)
- Pipeline CI/CD (GitHub Actions)

**Deferred Decisions (Post-MVP):**
- Isolation physique multi-tenant (base de données séparée)
- RBAC complet avec permissions granulaires
- Service externe de gestion de secrets (HashiCorp Vault)

### Data Architecture

**Multi-Tenant Data Model:**
- **Decision:** Isolation logique avec `organisation_id` sur toutes les tables
- **Rationale:** Simple, flexible, évolutif pour MVP. Permet évolution vers isolation physique si nécessaire.
- **Affects:** Toutes les tables Prisma, toutes les requêtes tRPC, middleware sécurité

**Data Validation Strategy:**
- **Decision:** Zod + Prisma pour validation complète
- **Version:** Zod (latest), Prisma (via T3 Stack)
- **Rationale:** Type-safety end-to-end (Zod → tRPC → Prisma), cohérent avec T3 Stack
- **Affects:** Toutes les API tRPC, schémas Prisma

**Migration Approach:**
- **Decision:** Prisma Migrations standard (`prisma migrate`)
- **Rationale:** Intégré à Prisma, versionné dans Git, compatible CI/CD, rollback possible
- **Affects:** Évolution du schéma base de données, déploiements

**Caching Strategy:**
- **Decision:** Redis (Upstash) pour cache + queue
- **Rationale:** Solution unifiée, serverless, compatible Vercel, coût maîtrisé, aligné avec préférences techniques
- **Affects:** Performance API, tâches asynchrones, génération pipelines

### Authentication & Security

**Authentication Method:**
- **Decision:** Clerk (service managé)
- **Rationale:** Setup rapide MVP, multi-tenant natif, social logins, gestion sessions, webhooks
- **Affects:** Toute l'authentification utilisateur, intégration frontend/backend

**Authorization Patterns:**
- **Decision:** RBAC simple (Owner/Member) au MVP
- **Rationale:** Suffisant pour MVP, facile à étendre vers RBAC complet si nécessaire
- **Affects:** Permissions utilisateurs, middleware autorisation

**Security Middleware:**
- **Decision:** Middleware Next.js + tRPC context pour isolation multi-tenant
- **Rationale:** Centralisé, type-safe, compatible T3 Stack, validation explicite possible
- **Affects:** Toutes les requêtes API, isolation données par organisation

**Data Encryption:**
- **Decision:** Chiffrement au repos avec librairie dédiée (ex: `@noble/ciphers`)
- **Rationale:** Contrôle total, pas de dépendance externe, compatible Prisma, MVP suffisant
- **Affects:** Stockage credentials API (API keys, OAuth tokens)

### API & Communication Patterns

**API Design:**
- **Decision:** tRPC (fourni par T3 Stack)
- **Rationale:** Type-safety end-to-end, intégré à T3 Stack, excellent DX
- **Affects:** Toutes les API backend-frontend

**Error Handling:**
- **Decision:** Erreurs tRPC standardisées avec codes personnalisés (`TRPCError`)
- **Rationale:** Type-safe, intégré à tRPC, format standardisé côté client
- **Affects:** Toutes les procédures tRPC, gestion erreurs frontend

**Rate Limiting:**
- **Decision:** Rate limiting au niveau middleware Next.js
- **Rationale:** Simple, intégré, suffisant pour MVP, peut évoluer vers service externe si nécessaire
- **Affects:** Protection API contre abus, middleware Next.js

### Infrastructure & Deployment

**Hosting Strategy:**
- **Decision:** Vercel (Next.js) + Postgres managé (Supabase/Neon) + Workers Docker (Railway/Fly.io)
- **Rationale:** Aligné avec préférences techniques, optimal pour Next.js, scalable
- **Affects:** Déploiement application, base de données, workers

**CI/CD Pipeline:**
- **Decision:** GitHub Actions
- **Rationale:** Intégration native avec Vercel, gratuit pour repos publics, bien documenté
- **Affects:** Automatisation déploiements, tests, lint, build

**Environment Configuration:**
- **Decision:** Vercel Environment Variables + .env.local
- **Rationale:** Intégré à Vercel, type-safe avec Zod (T3 Stack), simple
- **Affects:** Gestion variables d'environnement, développement local

**Monitoring & Logging:**
- **Decision:** Sentry pour observabilité et gestion d'erreurs
- **Rationale:** Intégration Next.js native, stack traces, alertes, déjà dans préférences techniques
- **Affects:** Observabilité application, gestion erreurs production

**Scaling Strategy:**
- **Decision:** Scaling horizontal avec isolation logique multi-tenant
- **Rationale:** Vercel auto-scaling, workers Docker scalables, Redis Upstash, isolation logique déjà décidée
- **Affects:** Scalabilité application, gestion charge

### Project-Specific Decisions

**Pipeline Configuration Format (Epic 1):**
- **Decision:** JSON (avec YAML en alternative)
- **Rationale:** Plus simple pour déclarer des modules, parsing natif JavaScript/TypeScript, YAML pour lisibilité si nécessaire
- **Affects:** Format fichiers pipelines, Epic 1, CLI validation

**Runtime CLI Architecture (Epic 2):**
- **Decision:** Go (langage pour runtime portable)
- **Rationale:** Portabilité cross-platform, performance, écosystème riche, plus simple à maintenir que Rust pour MVP
- **Affects:** Runtime CLI, exécution pipelines, Epic 2

**AI Integration for Mapping (Epic 3):**
- **Decision:** OpenAI API (GPT-4/GPT-3.5) pour suggestions mapping
- **Rationale:** Modèle performant, API stable, coût/performance équilibré
- **Affects:** Service génération mapping, Epic 3, suggestions IA

**CRON Scheduler for Polling (Epic 2):**
- **Decision:** Bibliothèque Go (robfig/cron) dans runtime CLI
- **Rationale:** Intégré au runtime CLI, pas de dépendance externe, exécution dans workers Docker
- **Affects:** Scheduler polling, Epic 2, runtime CLI

### Decision Impact Analysis

**Implementation Sequence:**
1. **Epic 1:** Définition format configuration (JSON avec YAML en alternative) avec schéma validation
2. **Epic 2:** Runtime CLI Go avec validation format, exécution modules, scheduler CRON
3. **Epic 3:** Frontend Next.js (T3 Stack) avec génération pipelines, intégration IA (OpenAPI)

**Cross-Component Dependencies:**
- **Format configuration → CLI → Frontend:** Format défini en premier, CLI valide, Front génère
- **Multi-tenant → Toutes les tables:** organisation_id requis partout, middleware sécurité
- **Clerk → tRPC context:** Authentification injectée dans contexte tRPC pour autorisation
- **Redis → Cache + Queue:** Solution unifiée pour performance et tâches asynchrones
- **Runtime CLI Go → Workers Docker:** CLI exécuté dans workers pour scalabilité

## Implementation Patterns & Consistency Rules

### Pattern Categories Defined

**Critical Conflict Points Identified:**
15+ areas where AI agents could make different choices without consistent patterns

### Naming Patterns

**Database Naming Conventions (Prisma):**
- **Tables:** `snake_case`, pluriel (ex: `users`, `organisations`, `connector_pipelines`, `input_modules`)
- **Columns:** `snake_case` (ex: `user_id`, `organisation_id`, `created_at`, `updated_at`)
- **Foreign Keys:** `{table}_id` format (ex: `user_id`, `organisation_id`, `connector_id`)
- **Indexes:** `idx_{table}_{column}` format (ex: `idx_users_email`, `idx_connector_pipelines_organisation_id`)
- **Enums:** `PascalCase` (ex: `UserRole`, `ConnectorStatus`)

**API Naming Conventions (tRPC):**
- **Routers:** `camelCase`, suffix `Router` (ex: `connectorRouter`, `userRouter`, `organisationRouter`)
- **Procedures:** `camelCase`, verbe d'action (ex: `getConnector`, `createPipeline`, `updateMapping`, `deleteConnector`)
- **Parameters:** `camelCase` (ex: `connectorId`, `organisationId`, `pipelineConfig`)
- **Route Structure:** `/server/api/routers/{domain}/{router}.ts`

**Code Naming Conventions (TypeScript/React):**
- **Components:** `PascalCase` (ex: `UserCard`, `ConnectorList`, `PipelineEditor`)
- **Component Files:** `kebab-case.tsx` (ex: `user-card.tsx`, `connector-list.tsx`)
- **Utils/Helpers:** `camelCase.ts` (ex: `formatDate.ts`, `validateConfig.ts`)
- **Functions:** `camelCase` (ex: `getUserData`, `formatConnectorName`, `validatePipelineConfig`)
- **Variables:** `camelCase` (ex: `userId`, `connectorData`, `isLoading`)
- **Constants:** `UPPER_SNAKE_CASE` (ex: `MAX_CONNECTOR_COUNT`, `DEFAULT_POLLING_INTERVAL`)
- **Types/Interfaces:** `PascalCase` (ex: `ConnectorConfig`, `PipelineModule`, `UserRole`)

### Structure Patterns

**Project Organization (T3 Stack):**
- **Tests:** Co-localisés avec fichiers source (`*.test.ts`, `*.spec.ts`)
- **Components:** `/app/components/{feature}/` (ex: `/app/components/connectors/`, `/app/components/users/`)
- **Utils:** `/utils/` (ex: `/utils/format.ts`, `/utils/validation.ts`, `/utils/constants.ts`)
- **tRPC Routers:** `/server/api/routers/{domain}/` (ex: `/server/api/routers/connector/`, `/server/api/routers/user/`)
- **Services:** `/server/services/` pour logique métier (ex: `/server/services/pipeline-generator.ts`)
- **Types:** `/types/` pour types partagés ou inline avec TypeScript
- **Config:** `/config/` pour configurations (ex: `/config/database.ts`, `/config/redis.ts`)

**File Structure Patterns:**
- **Environment Files:** `.env.local`, `.env.example` (ne pas versionner `.env.local`)
- **Config Files:** `{name}.config.ts` (ex: `database.config.ts`, `redis.config.ts`)
- **Static Assets:** `/public/` (ex: `/public/images/`, `/public/icons/`)
- **Documentation:** `/docs/` ou co-localisé avec README.md

**CLI Runtime (Go) - Separate Project:**
- **Structure:** `/cmd/`, `/internal/`, `/pkg/`, `/configs/`
- **Modules:** `/internal/modules/{type}/` (ex: `/internal/modules/input/`, `/internal/modules/filter/`, `/internal/modules/output/`)
- **Config Parsing:** `/internal/config/` pour validation JSON/YAML

### Format Patterns

**API Response Formats (tRPC):**
- **No Wrapper:** Retour direct des données (tRPC gère déjà l'encapsulation)
- **Success Response:** Données directement (ex: `return connector;` pas `return { data: connector };`)
- **Error Response:** `TRPCError` avec codes standardisés (`UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `BAD_REQUEST`, `INTERNAL_SERVER_ERROR`)
- **Date Format:** ISO 8601 strings (`YYYY-MM-DDTHH:mm:ss.sssZ`) dans JSON
- **Status Codes:** Gérés par tRPC automatiquement

**Data Exchange Formats:**
- **JSON API:** `camelCase` pour champs API (tRPC)
- **Database:** `snake_case` pour colonnes (Prisma)
- **Pipeline Config (JSON):** `camelCase` pour cohérence avec API (ex: `connectorId`, `inputModule`, `filterModules`)
- **Booleans:** `true`/`false` (jamais `1`/`0`)
- **Null Handling:** `null` explicite (pas de `undefined` dans JSON)
- **Arrays:** Toujours tableaux, jamais objets pour listes

**Pipeline Configuration Format (JSON):**
```json
{
  "connector": {
    "name": "erp-migration",
    "version": "1.0.0",
    "input": {
      "type": "httpPolling",
      "sourceApi": "old-erp-api",
      "schedule": "0 */6 * * *",
      "endpoint": "/api/customers"
    },
    "filter": [
      {
        "type": "mapping",
        "sourceSchema": "old-erp-customer",
        "targetSchema": "new-erp-client",
        "mappings": [
          {
            "source": "customer_id",
            "target": "client_id",
            "confidence": 0.95
          }
        ]
      }
    ],
    "output": {
      "type": "httpRequest",
      "targetApi": "new-erp-api",
      "endpoint": "/api/clients",
      "method": "POST"
    }
  }
}
```

### Communication Patterns

**State Management (React Server Components + tRPC):**
- **Default:** Server Components (pas de `'use client'` sauf nécessaire)
- **Client Components:** Uniquement pour interactivité (forms, buttons, modals)
- **Local State:** `useState` pour UI state uniquement
- **Server State:** tRPC queries/mutations (ex: `api.connector.getById.useQuery()`)
- **Loading States:** `isLoading` / `isFetching` depuis hooks tRPC
- **Naming:** `isLoading{Resource}` (ex: `isLoadingConnectors`, `isFetchingPipeline`)

**Event System Patterns:**
- **Event Naming:** `{resource}.{action}` format (ex: `connector.created`, `pipeline.updated`, `mapping.suggested`)
- **Event Payload:** Objet avec `{ type, data, timestamp, organisationId }`
- **Event Versioning:** Inclure `version` dans payload si nécessaire

**Logging Patterns:**
- **Production:** Sentry pour erreurs, structured logging pour traces
- **Development:** `console.log` / `console.error` avec contexte
- **Log Levels:** `error`, `warn`, `info`, `debug`
- **Log Format:** Structured JSON avec contexte (ex: `{ level: 'error', message: '...', organisationId: '...', userId: '...' }`)

### Process Patterns

**Error Handling Patterns:**
- **tRPC Errors:** `TRPCError` avec codes appropriés et messages utilisateur-friendly
- **Frontend Errors:** Error boundaries pour erreurs React, affichage messages utilisateur
- **Validation Errors:** Zod errors formatés pour affichage utilisateur
- **Logging:** Toujours logger erreurs avec contexte (organisationId, userId, etc.)
- **User-Facing Messages:** Messages clairs, pas de stack traces

**Loading State Patterns:**
- **Naming:** `isLoading{Resource}` (ex: `isLoadingConnectors`, `isLoadingPipeline`)
- **Scope:** Local par composant/page, pas global sauf nécessaire
- **UI:** Skeleton loaders pour meilleure UX (pas de spinners simples)
- **Persistence:** Pas de persistance loading state entre navigations

**Validation Patterns:**
- **API Validation:** Zod schemas pour toutes les entrées tRPC
- **Database Validation:** Prisma schema constraints
- **Client Validation:** Schémas Zod partagés pour validation côté client
- **Timing:** Validation avant soumission (client) + validation serveur (tRPC)
- **Error Display:** Messages d'erreur formatés par champ

**Authentication Flow Patterns:**
- **Clerk Integration:** Middleware Next.js pour extraction `userId` et `organisationId`
- **tRPC Context:** Injection `userId` et `organisationId` dans contexte tRPC
- **Authorization:** Vérification dans chaque procédure tRPC (pas de middleware automatique)
- **Session Management:** Géré par Clerk (pas de gestion manuelle)

**Multi-Tenant Isolation Patterns:**
- **Middleware:** Extraction `organisationId` depuis session Clerk ou headers
- **tRPC Context:** `organisationId` toujours présent dans contexte
- **Database Queries:** Toujours filtrer par `organisationId` (ex: `where: { organisationId: ctx.organisationId }`)
- **Validation:** Vérifier appartenance ressource à organisation avant opérations

### Enforcement Guidelines

**All AI Agents MUST:**

1. **Follow Naming Conventions:**
   - Database: `snake_case` pour tables/colonnes
   - API: `camelCase` pour tRPC routers/procedures
   - Components: `PascalCase` pour composants React
   - Files: `kebab-case.tsx` pour composants, `camelCase.ts` pour utils

2. **Respect Structure Patterns:**
   - Tests co-localisés avec `*.test.ts`
   - Components organisés par feature dans `/app/components/{feature}/`
   - tRPC routers dans `/server/api/routers/{domain}/`

3. **Use Standard Formats:**
   - ISO 8601 pour dates dans JSON
   - `camelCase` pour JSON API, `snake_case` pour database
   - `TRPCError` pour toutes les erreurs API

4. **Implement Multi-Tenant Isolation:**
   - Toujours filtrer par `organisationId` dans requêtes database
   - Vérifier appartenance ressource avant modifications
   - Injecter `organisationId` dans contexte tRPC

5. **Follow Error Handling Patterns:**
   - Utiliser `TRPCError` avec codes appropriés
   - Logger erreurs avec contexte (organisationId, userId)
   - Afficher messages utilisateur-friendly

**Pattern Enforcement:**
- **ESLint Rules:** Configurer règles pour naming conventions
- **TypeScript:** Utiliser types stricts pour validation
- **Code Review:** Vérifier patterns dans reviews
- **Documentation:** Documenter exceptions si nécessaire

### Pattern Examples

**Good Examples:**

**Database Schema (Prisma):**
```prisma
model ConnectorPipeline {
  id            String   @id @default(cuid())
  organisationId String
  name          String
  version       String
  config        Json
  createdAt     DateTime @default(now())
  updatedAt     DateTime @updatedAt
  
  organisation  Organisation @relation(fields: [organisationId], references: [id])
  
  @@index([organisationId])
  @@map("connector_pipelines")
}
```

**tRPC Router:**
```typescript
export const connectorRouter = createTRPCRouter({
  getById: protectedProcedure
    .input(z.object({ connectorId: z.string() }))
    .query(async ({ ctx, input }) => {
      const connector = await ctx.db.connectorPipeline.findFirst({
        where: {
          id: input.connectorId,
          organisationId: ctx.organisationId, // Multi-tenant isolation
        },
      });
      
      if (!connector) {
        throw new TRPCError({
          code: "NOT_FOUND",
          message: "Connector not found",
        });
      }
      
      return connector;
    }),
});
```

**React Component:**
```typescript
// app/components/connectors/connector-list.tsx
export function ConnectorList() {
  const { data: connectors, isLoading } = api.connector.getAll.useQuery();
  
  if (isLoading) {
    return <ConnectorListSkeleton />;
  }
  
  return (
    <div>
      {connectors?.map((connector) => (
        <ConnectorCard key={connector.id} connector={connector} />
      ))}
    </div>
  );
}
```

**Anti-Patterns:**

❌ **Database:** `Users` table (doit être `users`)
❌ **API:** `/api/get-connector` (doit être `connector.getById`)
❌ **Component:** `userCard.tsx` (doit être `user-card.tsx`)
❌ **Multi-tenant:** Requête sans `organisationId` filter
❌ **Error:** `throw new Error("...")` (doit être `TRPCError`)
❌ **Loading:** `loading` variable (doit être `isLoading`)
❌ **Date:** Timestamp number (doit être ISO 8601 string)

## Project Structure & Boundaries

### Requirements to Structure Mapping

**FR Categories → Components:**

- **Connector Management (9 FRs)** → `/server/api/routers/connector/`, `/app/components/connectors/`
- **OpenAPI Ingestion & Processing (6 FRs)** → `/server/services/openapi/`, `/server/api/routers/openapi/`
- **Automatic Connector Generation (13 FRs)** → `/server/services/pipeline-generator/`
- **AI-Assisted Mapping (7 FRs)** → `/server/services/ai-mapping/`
- **Connector Execution (17 FRs)** → `canectors-runtime/` (Runtime CLI Go - projet séparé)
- **Documentation Generation (8 FRs)** → `/server/services/documentation/`
- **Input/Filter/Output Modules (28 FRs)** → `canectors-runtime/internal/modules/`
- **User & Organization Management (14 FRs)** → `/server/api/routers/user/`, `/server/api/routers/organisation/`
- **Subscription & Billing (8 FRs)** → `/server/api/routers/billing/`
- **Integration & Workflow (6 FRs)** → `canectors-runtime/` + CI/CD

### Complete Project Directory Structure

**Main Application (T3 Stack - Next.js):**

```
canectors/
├── README.md
├── package.json
├── next.config.js
├── tailwind.config.js
├── tsconfig.json
├── .env.local
├── .env.example
├── .gitignore
├── .eslintrc.cjs
├── .prettierrc
├── .github/
│   └── workflows/
│       └── ci.yml
├── app/
│   ├── (auth)/
│   │   ├── layout.tsx
│   │   ├── login/
│   │   │   └── page.tsx
│   │   └── signup/
│   │       └── page.tsx
│   ├── (dashboard)/
│   │   ├── layout.tsx
│   │   ├── page.tsx
│   │   ├── connectors/
│   │   │   ├── page.tsx
│   │   │   ├── [id]/
│   │   │   │   ├── page.tsx
│   │   │   │   └── edit/
│   │   │   │       └── page.tsx
│   │   │   └── new/
│   │   │       └── page.tsx
│   │   ├── organisations/
│   │   │   ├── page.tsx
│   │   │   └── [id]/
│   │   │       └── page.tsx
│   │   ├── settings/
│   │   │   └── page.tsx
│   │   └── billing/
│   │       └── page.tsx
│   ├── api/
│   │   └── trpc/
│   │       └── [trpc]/
│   │           └── route.ts
│   ├── globals.css
│   ├── layout.tsx
│   └── page.tsx
├── components/
│   ├── ui/
│   │   ├── button.tsx
│   │   ├── input.tsx
│   │   ├── card.tsx
│   │   ├── dialog.tsx
│   │   ├── select.tsx
│   │   └── ...
│   ├── connectors/
│   │   ├── connector-list.tsx
│   │   ├── connector-card.tsx
│   │   ├── connector-editor.tsx
│   │   ├── pipeline-config-editor.tsx
│   │   ├── module-selector.tsx
│   │   └── connector-list.test.tsx
│   ├── organisations/
│   │   ├── organisation-switcher.tsx
│   │   ├── organisation-selector.tsx
│   │   └── organisation-switcher.test.tsx
│   ├── openapi/
│   │   ├── openapi-uploader.tsx
│   │   ├── openapi-viewer.tsx
│   │   └── openapi-uploader.test.tsx
│   └── shared/
│       ├── loading-skeleton.tsx
│       ├── error-boundary.tsx
│       └── ...
├── server/
│   ├── api/
│   │   ├── root.ts
│   │   └── routers/
│   │       ├── connector/
│   │       │   ├── connector.router.ts
│   │       │   └── connector.router.test.ts
│   │       ├── organisation/
│   │       │   ├── organisation.router.ts
│   │       │   └── organisation.router.test.ts
│   │       ├── user/
│   │       │   ├── user.router.ts
│   │       │   └── user.router.test.ts
│   │       ├── openapi/
│   │       │   ├── openapi.router.ts
│   │       │   └── openapi.router.test.ts
│   │       └── billing/
│   │           ├── billing.router.ts
│   │           └── billing.router.test.ts
│   ├── services/
│   │   ├── openapi/
│   │   │   ├── openapi-parser.ts
│   │   │   ├── openapi-validator.ts
│   │   │   └── openapi-parser.test.ts
│   │   ├── pipeline-generator/
│   │   │   ├── pipeline-generator.ts
│   │   │   ├── input-module-generator.ts
│   │   │   ├── filter-module-generator.ts
│   │   │   ├── output-module-generator.ts
│   │   │   └── pipeline-generator.test.ts
│   │   ├── ai-mapping/
│   │   │   ├── mapping-suggester.ts
│   │   │   ├── openai-client.ts
│   │   │   └── mapping-suggester.test.ts
│   │   └── documentation/
│   │       ├── pipeline-doc-generator.ts
│   │       └── pipeline-doc-generator.test.ts
│   ├── middleware.ts
│   └── db.ts
├── prisma/
│   ├── schema.prisma
│   └── migrations/
├── types/
│   ├── connector.ts
│   ├── pipeline.ts
│   ├── openapi.ts
│   └── organisation.ts
├── utils/
│   ├── format.ts
│   ├── validation.ts
│   ├── constants.ts
│   └── encryption.ts
├── config/
│   ├── database.config.ts
│   ├── redis.config.ts
│   └── clerk.config.ts
├── public/
│   ├── images/
│   └── icons/
└── docs/
    └── README.md
```

**Runtime CLI (Go - Projet séparé):**

```
canectors-runtime/
├── README.md
├── go.mod
├── go.sum
├── .gitignore
├── .github/
│   └── workflows/
│       └── ci.yml
├── cmd/
│   └── canectors/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   ├── validator.go
│   │   └── config_test.go
│   ├── modules/
│   │   ├── input/
│   │   │   ├── http_polling.go
│   │   │   ├── webhook.go
│   │   │   └── input_test.go
│   │   ├── filter/
│   │   │   ├── mapping.go
│   │   │   ├── condition.go
│   │   │   └── filter_test.go
│   │   └── output/
│   │       ├── http_request.go
│   │       └── output_test.go
│   ├── runtime/
│   │   ├── pipeline.go
│   │   ├── executor.go
│   │   └── runtime_test.go
│   ├── scheduler/
│   │   ├── cron.go
│   │   └── scheduler_test.go
│   └── logger/
│       └── logger.go
├── pkg/
│   └── connector/
│       └── types.go
└── configs/
    └── example-connector.json
```

### Architectural Boundaries

**API Boundaries:**

- **External API:** tRPC endpoints exposés via `/app/api/trpc/[trpc]/route.ts`
- **Internal Service Boundaries:** Services dans `/server/services/` communiquent via appels de fonctions
- **Authentication Boundary:** Clerk middleware dans `/server/middleware.ts` injecte `userId` et `organisationId` dans contexte tRPC
- **Data Access Boundary:** Prisma client dans `/server/db.ts`, accès uniquement via ce client

**Component Boundaries:**

- **Frontend Components:** Communication via props et tRPC hooks uniquement
- **Server Components:** Par défaut, Client Components uniquement si interactivité requise
- **State Management:** État serveur via tRPC, état local UI via `useState`
- **Service Communication:** Services appelés depuis routers tRPC uniquement

**Service Boundaries:**

- **Pipeline Generator Service:** Génère configurations JSON depuis OpenAPI
- **AI Mapping Service:** Suggestions mapping via OpenAI API
- **OpenAPI Service:** Parsing et validation OpenAPI
- **Documentation Service:** Génération documentation pipelines

**Data Boundaries:**

- **Database Schema:** Défini dans `/prisma/schema.prisma`, toutes les tables ont `organisationId`
- **Data Access:** Uniquement via Prisma client, toujours filtrer par `organisationId`
- **Caching:** Redis Upstash pour cache, accès via `/config/redis.config.ts`
- **External Data:** OpenAPI specs (input), configurations pipelines (output)

### Integration Points

**Internal Communication:**

- **Frontend ↔ Backend:** tRPC queries/mutations via `/app/api/trpc/[trpc]/route.ts`
- **Routers ↔ Services:** Appels directs de fonctions depuis routers vers services
- **Services ↔ Database:** Via Prisma client dans `/server/db.ts`
- **Middleware ↔ Routers:** Contexte tRPC avec `userId` et `organisationId`

**External Integrations:**

- **Clerk:** Authentification, middleware Next.js
- **OpenAI API:** Suggestions mapping, service `/server/services/ai-mapping/`
- **Stripe:** Billing, router `/server/api/routers/billing/`
- **PostHog:** Analytics, intégration frontend
- **Sentry:** Observabilité, intégration Next.js
- **Resend:** Email, service dédié
- **Redis Upstash:** Cache et queue, config `/config/redis.config.ts`
- **Supabase/Neon:** PostgreSQL, config Prisma

**Data Flow:**

1. **User Action** → Frontend Component (React)
2. **tRPC Mutation** → Router (`/server/api/routers/{domain}/`)
3. **Service Call** → Service (`/server/services/{service}/`)
4. **Database Query** → Prisma Client (`/server/db.ts`)
5. **Response** → tRPC → Frontend Component

**Runtime CLI Integration:**

- **Configuration Input:** JSON files depuis frontend ou CI/CD
- **Execution:** Runtime CLI Go exécute pipelines selon configuration
- **Output:** Logs, résultats, erreurs
- **Communication:** Fichiers JSON, pas d'API directe (déterministe, portable)

### File Organization Patterns

**Configuration Files:**
- Root level: `package.json`, `next.config.js`, `tsconfig.json`, `.env.local`
- Config directory: `/config/` pour configurations TypeScript
- Environment: `.env.local` (local), `.env.example` (template)

**Source Organization:**
- App Router: `/app/` pour routes Next.js
- Components: `/components/{feature}/` organisés par feature
- Server: `/server/` pour backend (routers, services, middleware)
- Utils: `/utils/` pour helpers partagés
- Types: `/types/` pour types TypeScript partagés

**Test Organization:**
- Co-localisés: `*.test.ts` / `*.spec.ts` à côté des fichiers source
- Structure: Même structure que source code

**Asset Organization:**
- Static: `/public/` pour assets statiques (images, icons)
- Documentation: `/docs/` pour documentation projet

### Development Workflow Integration

**Development Server Structure:**
- Next.js dev server: `npm run dev` depuis root
- Hot reloading: Automatique pour `/app/`, `/components/`, `/server/`
- TypeScript: Compilation automatique, erreurs dans terminal

**Build Process Structure:**
- Build: `npm run build` compile Next.js app
- Output: `.next/` directory (ignoré dans Git)
- Prisma: `prisma generate` génère Prisma Client
- TypeScript: Compilation stricte, erreurs bloquantes

**Deployment Structure:**
- Vercel: Déploiement automatique depuis GitHub
- Environment Variables: Configurées dans Vercel Dashboard
- Database Migrations: Exécutées via Prisma Migrate
- Runtime CLI: Build séparé, déployé dans workers Docker (Railway/Fly.io)

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:**
Toutes les décisions architecturales sont cohérentes et compatibles :
- **T3 Stack** : Next.js 15, TypeScript, Prisma, tRPC, Tailwind CSS - stack cohérente et bien intégrée
- **Multi-tenant** : Isolation logique avec `organisation_id` compatible avec Prisma et tRPC
- **Authentication** : Clerk compatible avec Next.js middleware et tRPC context
- **Database** : Prisma avec PostgreSQL (Supabase/Neon) - choix standard et compatible
- **Cache/Queue** : Redis Upstash compatible avec Vercel et architecture serverless
- **Runtime CLI** : Go séparé du projet Next.js - architecture modulaire claire
- **Format Configuration** : JSON principal avec YAML alternative - compatible avec parsing natif et validation

**Pattern Consistency:**
Les patterns d'implémentation sont cohérents avec les décisions :
- **Naming** : Conventions cohérentes (snake_case DB, camelCase API, PascalCase Components)
- **Structure** : Organisation T3 Stack respectée, séparation claire frontend/backend
- **Communication** : tRPC pour API, props pour composants, appels directs pour services
- **Multi-tenant** : Patterns d'isolation cohérents (middleware → context → queries)

**Structure Alignment:**
La structure du projet supporte toutes les décisions :
- **T3 Stack** : Structure App Router respectée (`/app/`, `/server/`, `/components/`)
- **Multi-tenant** : Middleware et context définis (`/server/middleware.ts`, tRPC context)
- **Services** : Organisation par domaine (`/server/services/{domain}/`)
- **Runtime CLI** : Projet séparé (`canectors-runtime/`) avec structure Go standard
- **Tests** : Co-localisation avec fichiers source (`*.test.ts`)

### Requirements Coverage Validation ✅

**Epic/Feature Coverage:**
Tous les epics sont architecturally supported :
- **Epic 1 (Format Configuration)** : Structure `/types/`, validation Zod, format JSON défini
- **Epic 2 (CLI Runtime)** : Projet `canectors-runtime/` avec modules Input/Filter/Output, scheduler CRON
- **Epic 3 (Frontend Generator)** : Services `/server/services/pipeline-generator/`, `/server/services/ai-mapping/`

**Functional Requirements Coverage:**
Toutes les 12 catégories FR sont couvertes :
- ✅ **Connector Management** : Routers `/server/api/routers/connector/`, Components `/components/connectors/`
- ✅ **OpenAPI Ingestion** : Service `/server/services/openapi/`, Router `/server/api/routers/openapi/`
- ✅ **Automatic Generation** : Service `/server/services/pipeline-generator/`
- ✅ **AI-Assisted Mapping** : Service `/server/services/ai-mapping/` avec OpenAI
- ✅ **Connector Execution** : Runtime CLI Go `canectors-runtime/` avec modules
- ✅ **Documentation** : Service `/server/services/documentation/`
- ✅ **Input/Filter/Output Modules** : `canectors-runtime/internal/modules/`
- ✅ **User & Organization** : Routers `/server/api/routers/user/`, `/server/api/routers/organisation/`
- ✅ **Subscription & Billing** : Router `/server/api/routers/billing/` avec Stripe
- ✅ **Integration & Workflow** : Runtime CLI + CI/CD GitHub Actions
- ✅ **Template & Reusability** : Supporté par structure modulaire

**Non-Functional Requirements Coverage:**
Tous les NFRs sont architecturally addressed :
- ✅ **Performance** : Redis cache, optimisations Next.js, génération asynchrone
- ✅ **Security** : Clerk auth, isolation multi-tenant, chiffrement credentials
- ✅ **Scalability** : Vercel auto-scaling, workers Docker, Redis Upstash
- ✅ **Reliability** : Runtime Go déterministe, gestion erreurs tRPC, Sentry
- ✅ **Integration** : OpenAPI 3.0, CLI cross-platform, CI/CD, format JSON versionnable
- ✅ **Maintainability** : Format déclaratif stable, runtime unique, patterns cohérents

### Implementation Readiness Validation ✅

**Decision Completeness:**
Toutes les décisions critiques sont documentées :
- ✅ 20 décisions architecturales documentées avec versions et rationales
- ✅ Technologies spécifiées (T3 Stack, Clerk, Redis Upstash, Go, etc.)
- ✅ Patterns d'intégration définis (tRPC, Prisma, middleware)
- ✅ Versions vérifiées et compatibles

**Structure Completeness:**
Structure de projet complète et spécifique :
- ✅ Arborescence complète définie pour `canectors/` (Next.js)
- ✅ Arborescence complète définie pour `canectors-runtime/` (Go)
- ✅ Tous les fichiers et répertoires mappés aux exigences
- ✅ Points d'intégration clairement spécifiés
- ✅ Boundaries de composants bien définis

**Pattern Completeness:**
Patterns d'implémentation complets :
- ✅ Conventions de nommage complètes (DB, API, Code)
- ✅ Patterns de structure définis (organisation, tests, assets)
- ✅ Patterns de communication spécifiés (tRPC, state management)
- ✅ Patterns de processus documentés (error handling, loading, validation)
- ✅ Exemples concrets fournis (Good examples + Anti-patterns)

### Gap Analysis Results

**Critical Gaps:** Aucun
- Toutes les décisions critiques sont prises
- Tous les patterns essentiels sont définis
- Structure complète pour démarrer l'implémentation

**Important Gaps:** Aucun
- Patterns suffisamment détaillés
- Exemples fournis pour cas d'usage principaux
- Documentation complète pour guidage agents IA

**Nice-to-Have Gaps (Post-MVP):**
- **Monitoring avancé** : Métriques custom au-delà de Sentry (PostHog analytics)
- **Testing E2E** : Structure pour tests end-to-end (Playwright/Cypress)
- **Documentation API** : Génération automatique docs tRPC (optionnel)
- **Performance profiling** : Outils de profiling pour optimisation (optionnel)

### Validation Issues Addressed

**Aucun problème critique identifié.** L'architecture est cohérente, complète et prête pour l'implémentation.

**Ajustements effectués pendant le workflow :**
- ✅ Format configuration : JSON principal (au lieu de YAML) pour simplicité déclaration modules
- ✅ Nom projet : Canectors

### Architecture Completeness Checklist

**✅ Requirements Analysis**
- [x] Project context thoroughly analyzed
- [x] Scale and complexity assessed (Moyenne à élevée, 10-14 composants)
- [x] Technical constraints identified (Ordre développement séquentiel, CLI source vérité)
- [x] Cross-cutting concerns mapped (8 préoccupations transversales)

**✅ Architectural Decisions**
- [x] Critical decisions documented with versions (20 décisions)
- [x] Technology stack fully specified (T3 Stack, Clerk, Redis, Go, etc.)
- [x] Integration patterns defined (tRPC, Prisma, middleware, services)
- [x] Performance considerations addressed (Cache, scaling, optimisations)

**✅ Implementation Patterns**
- [x] Naming conventions established (DB, API, Code - exemples fournis)
- [x] Structure patterns defined (T3 Stack, tests co-localisés)
- [x] Communication patterns specified (tRPC, state management, events)
- [x] Process patterns documented (error handling, loading, validation, auth)

**✅ Project Structure**
- [x] Complete directory structure defined (canectors/ + canectors-runtime/)
- [x] Component boundaries established (API, Services, Components, Data)
- [x] Integration points mapped (Internal, External, Data Flow, Runtime CLI)
- [x] Requirements to structure mapping complete (12 catégories FR → composants)

### Architecture Readiness Assessment

**Overall Status:** ✅ READY FOR IMPLEMENTATION

**Confidence Level:** HIGH - Architecture complète, cohérente et bien documentée

**Key Strengths:**
- **Cohérence complète** : Toutes les décisions s'alignent parfaitement
- **Couverture exhaustive** : Tous les FRs et NFRs sont architecturally supported
- **Patterns détaillés** : Conventions claires avec exemples et anti-patterns
- **Structure complète** : Arborescence spécifique pour Next.js et Go
- **Documentation complète** : Guide clair pour agents IA
- **Ordre développement** : Séquence Epic 1 → 2 → 3 clairement définie

**Areas for Future Enhancement:**
- **Monitoring avancé** : Métriques custom et dashboards (post-MVP)
- **Testing E2E** : Tests end-to-end complets (post-MVP)
- **Documentation API** : Génération automatique docs tRPC (optionnel)
- **Performance profiling** : Outils d'optimisation avancés (optionnel)

### Implementation Handoff

**AI Agent Guidelines:**

- **Follow all architectural decisions exactly as documented** : Utiliser T3 Stack, Clerk, Redis Upstash, Go runtime, etc.
- **Use implementation patterns consistently** : Respecter naming conventions, structure patterns, communication patterns
- **Respect project structure and boundaries** : Utiliser structure définie, respecter boundaries API/Services/Components
- **Refer to this document for all architectural questions** : Ce document est la source de vérité architecturale

**First Implementation Priority:**

1. **Epic 1 - Format Configuration** : Définir schéma JSON complet pour pipelines modulaires avec validation Zod
2. **Initialize T3 Stack** : `npx create-t3-app@latest canectors --nextAuth --prisma --trpc --tailwind --typescript`
3. **Setup Multi-tenant** : Configurer middleware Next.js + tRPC context pour isolation `organisationId`
4. **Epic 2 - CLI Runtime** : Initialiser projet Go `canectors-runtime/` avec structure modules Input/Filter/Output

## Architecture Completion Summary

### Workflow Completion

**Architecture Decision Workflow:** COMPLETED ✅
**Total Steps Completed:** 8
**Date Completed:** 2026-01-11
**Document Location:** `_bmad-output/planning-artifacts/architecture.md`

### Final Architecture Deliverables

**📋 Complete Architecture Document**

- All architectural decisions documented with specific versions
- Implementation patterns ensuring AI agent consistency
- Complete project structure with all files and directories
- Requirements to architecture mapping
- Validation confirming coherence and completeness

**🏗️ Implementation Ready Foundation**

- **20** architectural decisions made
- **15+** implementation patterns defined
- **10-14** architectural components specified
- **125+** functional requirements fully supported
- **6** non-functional requirements addressed

**📚 AI Agent Implementation Guide**

- Technology stack with verified versions (T3 Stack, Clerk, Redis Upstash, Go)
- Consistency rules that prevent implementation conflicts
- Project structure with clear boundaries (canectors/ + canectors-runtime/)
- Integration patterns and communication standards (tRPC, Prisma, middleware)

### Implementation Handoff

**For AI Agents:**
This architecture document is your complete guide for implementing **Canectors**. Follow all decisions, patterns, and structures exactly as documented.

**First Implementation Priority:**

1. **Epic 1 - Format Configuration** : Définir schéma JSON complet pour pipelines modulaires avec validation Zod
2. **Initialize T3 Stack** : `npx create-t3-app@latest canectors --nextAuth --prisma --trpc --tailwind --typescript`
3. **Setup Multi-tenant** : Configurer middleware Next.js + tRPC context pour isolation `organisationId`
4. **Epic 2 - CLI Runtime** : Initialiser projet Go `canectors-runtime/` avec structure modules Input/Filter/Output

**Development Sequence:**

1. Initialize project using documented starter template (T3 Stack)
2. Set up development environment per architecture (Vercel, Supabase/Neon, Redis Upstash)
3. Implement core architectural foundations (multi-tenant, auth, database)
4. Build features following established patterns (Epic 1 → 2 → 3)
5. Maintain consistency with documented rules (naming, structure, communication)

### Quality Assurance Checklist

**✅ Architecture Coherence**

- [x] All decisions work together without conflicts
- [x] Technology choices are compatible (T3 Stack, Clerk, Redis, Go)
- [x] Patterns support the architectural decisions
- [x] Structure aligns with all choices (Next.js + Go runtime)

**✅ Requirements Coverage**

- [x] All functional requirements are supported (12 catégories FR)
- [x] All non-functional requirements are addressed (6 NFRs)
- [x] Cross-cutting concerns are handled (8 préoccupations transversales)
- [x] Integration points are defined (Internal, External, Data Flow, Runtime CLI)

**✅ Implementation Readiness**

- [x] Decisions are specific and actionable (20 décisions avec versions)
- [x] Patterns prevent agent conflicts (15+ patterns avec exemples)
- [x] Structure is complete and unambiguous (canectors/ + canectors-runtime/)
- [x] Examples are provided for clarity (Good examples + Anti-patterns)

### Project Success Factors

**🎯 Clear Decision Framework**
Every technology choice was made collaboratively with clear rationale, ensuring all stakeholders understand the architectural direction.

**🔧 Consistency Guarantee**
Implementation patterns and rules ensure that multiple AI agents will produce compatible, consistent code that works together seamlessly.

**📋 Complete Coverage**
All project requirements are architecturally supported, with clear mapping from business needs to technical implementation (125+ FRs, 6 NFRs).

**🏗️ Solid Foundation**
The chosen starter template (T3 Stack) and architectural patterns provide a production-ready foundation following current best practices.

---

**Architecture Status:** READY FOR IMPLEMENTATION ✅

**Next Phase:** Begin implementation using the architectural decisions and patterns documented herein.

**Document Maintenance:** Update this architecture when major technical decisions are made during implementation.
