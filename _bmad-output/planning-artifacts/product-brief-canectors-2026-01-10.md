---
stepsCompleted: [1, 2, 3, 4, 5]
inputDocuments: ['research/market-api-connector-automation-saas-research-2026-01-10.md']
date: 2026-01-10
author: Cano
---

# Product Brief: Cannectors

## Executive Summary

**Solution SaaS B2B developer-first** qui automatise la génération de pipelines déclaratifs modulaires de connecteurs API entre systèmes métiers à partir de spécifications OpenAPI, avec IA assistive pour le mapping des données.

**Architecture technique :** Génération de pipelines déclaratifs modulaires (Input → Filter → Output) interprétés par un runtime portable et déterministe, avec IA utilisée uniquement pour assister la génération (pas l'exécution), laissant au développeur un contrôle total sur la logique métier, la sécurité et l'exécution.

**Objectif :** Réduire le temps de création de connecteurs de 2-5 jours à 1-4 heures pour un connecteur fonctionnel, et moins d'une journée pour un connecteur prêt pour la production.

**Positionnement :** Automatiser la partie la plus pénible (analyse des schémas OpenAPI, génération des configurations de pipelines déclaratifs avec étapes d'entrée, transformation et sortie) tout en conservant un runtime déterministe, explicable et maîtrisé par les développeurs.

**Différenciation :** Combinaison unique d'automatisation de génération de pipelines déclaratifs modulaires, d'IA assistive et de runtime portable avec contrôle total du développeur — une approche plus maintenable et flexible que la génération d'applications complètes (Spring Boot, etc.).

---

## Core Vision

### Problem Statement

**Problème :** Les développeurs backend spécialisés dans les intégrations et connecteurs API passent une part importante de leur temps à connecter des systèmes métiers (ERP, SaaS, APIs internes). Ce travail est répétitif, long et fragile, apporte peu de valeur métier directe, mais reste indispensable pour l'entreprise.

**Douleurs spécifiques à chaque projet :**
- **Compréhension des OpenAPI hétérogènes** : Analyse manuelle de spécifications variées et souvent incomplètes
- **Mapping des formats de données différents** : Transformation manuelle entre schémas (XML ↔ JSON, formats propriétaires)
- **Gestion des transformations complexes** : Logique de transformation custom pour chaque intégration
- **Maintenance continue** : Surveillance et mises à jour manuelles lors de changements d'API upstream
- **Documentation et transmission** : Explication et transfert de connaissances aux autres équipes

**Temps consacré aujourd'hui :** 2-5 jours par API pour des intégrations complexes, avec maintenance continue lors de changements d'API.

### Problem Impact

**Impact sur les développeurs :**
- Temps important passé sur des tâches répétitives plutôt que sur des fonctionnalités différenciantes
- Fatigue cognitive liée à la répétition des mêmes analyses et mappings
- Frustration face à la fragilité des intégrations et à la maintenance continue

**Impact sur les équipes :**
- Coût d'opportunité : Temps non consacré aux fonctionnalités métier différenciantes
- Skills gap : Nécessité d'expertise dans chaque API spécifique (auth, pagination, versioning) plutôt que dans le domaine métier
- Risque opérationnel : Intégrations fragiles qui cassent lors de changements d'API
- Friction organisationnelle : Difficultés à expliquer et maintenir les choix techniques aux équipes non-techniques

**Impact sur l'entreprise :**
- Coûts de développement élevés pour des intégrations critiques mais non différenciantes
- Délais de mise sur le marché allongés par la création manuelle de connecteurs
- Risque de vendor lock-in ou de dépendance à des solutions trop lourdes (iPaaS Enterprise) ou trop limitées (no-code)

### Why Existing Solutions Fall Short

**iPaaS traditionnels (MuleSoft, Workato, Zapier) :**
- **Trop lourds** : Setup, complexité fonctionnelle et coûts dépassent souvent la valeur réelle du connecteur pour des intégrations API fréquentes mais relativement simples
- **Trop no-code** : Limitation du contrôle, de la versionabilité et de l'intégration dans les workflows développeurs (Git, CI/CD) dès que le mapping ou la logique sort du happy path
- **Vendor lock-in majeur** : Workflows dans la plateforme, pas de code exportable/déployable
- **Peu orienté développeurs** : UI visuelles plutôt qu'outils pour développeurs, mapping de données manuel/difficile

**Outils Developer-First existants (OpenAPI Generator, Prismatic) :**
- **Génération partielle** : Génèrent des clients/stubs ou des abstractions déclaratives, mais pas un connecteur complet avec mapping métier, transformations et maintenance dans le temps
- **Mapping manuel** : Nécessitent encore un travail manuel important pour le mapping des données et les transformations
- **Pas d'intelligence** : Pas de capacité à proposer des mappings probables ou à expliquer des transformations
- **Pas d'orchestration** : Pas de gestion complète du cycle de vie du connecteur (génération, maintenance, monitoring)

**Solutions internes (custom) :**
- **Coût élevé de maintenance** : Logique de mapping et gestion d'erreurs dupliquée entre projets sans réutilisabilité
- **Skills gap** : Nécessité d'expertise dans chaque API spécifique plutôt que dans le domaine métier
- **Pas de standardisation** : Approches différentes selon les projets, difficulté à maintenir la cohérence

**Gap identifié :** Aucune solution ne combine automatisation complète (OpenAPI → configurations de pipelines production-ready), mapping intelligent avec IA assistive, et runtime portable avec contrôle total du développeur.

### Proposed Solution

**Vision produit :** Plateforme developer-first permettant de générer et d'exécuter des pipelines déclaratifs modulaires de connecteurs API entre systèmes métiers à partir de leurs spécifications OpenAPI, avec IA assistive pour le mapping des données et les transformations.

**Architecture technique :**
- **Pipelines déclaratifs modulaires** : Format DSL/configuration explicite (YAML/JSON/Toml ou format custom) définissant des pipelines composés d'étapes (entrée, transformation, sortie) suivant le pattern Input → Filter → Output
- **Runtime portable et déterministe** : Runtime unique interprétant et exécutant les configurations de pipelines, portable (Go/Rust ou autre), déterministe et versionné
- **IA assistive pour génération uniquement** : IA utilisée uniquement pour assister la génération des configurations de pipelines (suggestions de mapping, détection de patterns), pas pour l'exécution
- **Contrôle développeur total** : Le développeur garde le contrôle complet sur la logique métier, la sécurité et l'exécution via les configurations de pipelines éditable et intégrables dans les outils existants des équipes

**Objectif :** Réduire le temps de création de connecteurs de 2-5 jours à 1-4 heures pour un connecteur fonctionnel, et moins d'une journée pour un connecteur prêt pour la production après validation.

**Philosophie :** Ne pas remplacer les développeurs, mais leur faire gagner un temps considérable en automatisant la partie la plus pénible (analyse des schémas OpenAPI, génération des configurations de pipelines déclaratifs avec mapping et transformations), tout en conservant un runtime déterministe, explicable et maîtrisé.

**Avantages de cette approche déclarative :**
- **Simplicité maintenance** : Runtime unique à maintenir (pas de dépendances frameworks externes)
- **Stabilité** : Format déclaratif stable, backward compatible, évolutif
- **Sécurité** : Runtime versionné et maintenu par la plateforme, pas de dépendances externes dans le code généré
- **Portabilité** : Runtime portable fonctionne partout (on-premise, cloud, CI/CD)
- **Contrôle développeur** : Configurations de pipelines éditable, testable par le développeur, s'intégrant naturellement dans les outils existants des équipes
- **Évolution stack** : Runtime peut évoluer indépendamment des configurations de pipelines, pas dépendant de frameworks externes

**Caractéristiques clés :**
- **Génération configurations pipelines complètes** : OpenAPI → configurations complètes de pipelines (auth, pagination, retry, mapping, gestion erreurs) avec étapes Input → Filter → Output
- **IA assistive génération** : Propose et explique des mappings et transformations probables (ex. customer_id ↔ client_id) dans les configurations de pipelines, mais l'exécution reste déterministe et validée par le développeur
- **Runtime portable** : Runtime unique, portable, déterministe, versionné et maintenu par la plateforme, exécutant les pipelines de manière déterministe
- **Contrôle total développeur** : Configurations de pipelines explicites, lisibles, éditable, testable par le développeur, s'intégrant naturellement dans les outils existants des équipes (versioning, CI/CD, etc.)
- **Intégration workflow développeur** : CLI moderne, plugins VSCode/CI-CD, fichiers standards s'intégrant dans les workflows existants
- **Maintenance automatisée** : Détection changements OpenAPI, re-génération automatique configurations de pipelines avec validation développeur, monitoring et observabilité

**Cas d'usage prioritaires :**
- **Pagination custom** : Configurations de pipelines permettant gestion automatique patterns de pagination différents via étapes Filter
- **Enums incompatibles** : Mapping intelligent entre énumérations différentes dans les configurations de pipelines
- **Formats propriétaires** : Transformations entre XML, JSON et formats propriétaires via étapes Filter dans les pipelines
- **Auth spécifiques** : Support différents mécanismes authentification (OAuth, API keys, custom headers) dans les configurations de pipelines
- **Évolutions fréquentes API** : Détection automatique changements OpenAPI, re-génération automatique configurations de pipelines avec validation développeur

**Objectif long terme :** Devenir l'outil de référence pour les intégrations API B2B, recommandé par les développeurs, avec des pipelines déclaratifs générés fiables et maintenables, et une intégration naturelle dans les workflows techniques existants.

### Key Differentiators

**Différenciation principale :** Combinaison unique d'automatisation de génération de configurations de pipelines déclaratifs, d'IA assistive et de runtime portable avec contrôle total du développeur — une approche plus maintenable et flexible que la génération d'applications complètes (Spring Boot, etc.).

**Différenciateurs spécifiques :**

1. **Génération pipelines déclaratifs vs. applications complètes**
   - Concurrents : Génèrent soit des applications complètes (complexe à maintenir, dépendances frameworks), soit des abstractions déclaratives trop limitées (manque de contrôle)
   - Notre solution : Génère des configurations de pipelines déclaratifs modulaires (Input → Filter → Output), explicites, lisibles, interprétées par un runtime portable et déterministe

2. **Runtime portable unique vs. code généré avec dépendances**
   - Concurrents : Génèrent du code avec dépendances externes (Spring Boot, Node.js, etc.) = maintenance complexe, risques sécurité
   - Notre solution : Runtime unique, portable, versionné et maintenu par la plateforme = maintenance simple, sécurité centralisée

3. **IA assistive génération uniquement (pas exécution)**
   - Concurrents : Mapping manuel ou règles fixes rigides
   - Notre solution : IA propose et explique des mappings probables (customer_id ↔ client_id) dans les configurations de pipelines, mais l'exécution reste déterministe et validée par le développeur

4. **Developer-first avec contrôle total**
   - Concurrents : Solutions "black box" ou applications générées non modifiables
   - Notre solution : Configurations de pipelines explicites, éditable, testable par le développeur, s'intégrant naturellement dans les outils existants = contrôle total logique métier, sécurité, exécution

5. **Évolution indépendante runtime vs. configurations**
   - Concurrents : Code généré dépendant de frameworks externes = problèmes si frameworks évoluent
   - Notre solution : Runtime peut évoluer indépendamment des configurations de pipelines, format déclaratif stable et backward compatible

6. **Simplicité maintenance vs. complexité dépendances**
   - Concurrents : Maintenance dépendances frameworks externes, risques sécurité multiples
   - Notre solution : Runtime unique à maintenir, pas de dépendances externes dans configurations de pipelines générées = maintenance simple, sécurité centralisée

**Avantage concurrentiel durable :** First-mover advantage sur automatisation génération pipelines déclaratifs modulaires + IA assistive + runtime portable avec contrôle développeur, avec focus développeurs et intégration workflow naturelle créant des switching costs positifs (configurations exportables) et des network effects (templates communautaires partagés).

---

## Target Users

### Primary Users

#### Persona 1 : Marc — Senior Integration Engineer / Consultant ERP

**Nom et contexte :**
- **Nom :** Marc
- **Rôle :** Senior Integration Engineer / Consultant ERP
- **Contexte :** Marc travaille pour une ESN spécialisée en intégration et intervient chez des clients pour des projets de migration ERP (ERP legacy → ERP SaaS ou ERP A → ERP B)
- **Type d'organisation cliente :** PME Tech ou scale-up (100 à 500 employés), avec un SI structuré mais sous pression de croissance
- **Contexte professionnel :** Marc n'est pas employé du client final, mais mandaté pour livrer un résultat, avec un engagement fort sur les délais et la fiabilité

**Contraintes et contexte :**
- **Contrainte de délais :** Projets avec deadlines serrées, souvent liées à un go-live ERP, une fin de contrat, ou une migration planifiée sur quelques semaines. Chaque jour de retard a un impact direct sur la facturation, la crédibilité de l'ESN et la satisfaction du client
- **Type de projet :** Migration ERP (clients, commandes, produits, factures), synchronisation temporaire entre deux systèmes pendant la transition, intégration critique mais limitée dans le temps
- **Contrainte de fiabilité :** Les données manipulées sont critiques pour le business — erreurs = commandes perdues, facturation incorrecte, données clients incohérentes. La migration est souvent auditée après coup
- **Contrôle total critique :** Marc doit pouvoir expliquer exactement ce qui a été transformé et pourquoi, justifier un mapping à un client ou à un auditeur, garantir que le comportement est reproductible. Les solutions "black box" sont inacceptables (pas d'audit trail clair, pas de confiance du client, risque contractuel pour l'ESN)

**Workflow actuel :**
- **Analyse des APIs :** Lit la documentation (souvent incomplète), explore les OpenAPI quand elles existent, utilise Postman pour tester les endpoints, fait parfois du reverse engineering via les réponses réelles. Processus long, répétitif et source d'erreurs
- **Mapping des données :** Fait via tableaux Excel partagés avec le client, scripts custom (Java, Python, Node), règles codées "à la main" dans le code d'intégration. Chaque projet recommence quasiment de zéro
- **Synchronisations / exécution :** Exécute via scripts Python ou Java ad hoc, services Spring Boot temporaires, jobs lancés manuellement ou via cron. La gestion des erreurs et des retries est souvent basique
- **Livraison finale :** Doit livrer du code fonctionnel (scripts ou service), une documentation du mapping, parfois une solution que le client devra maintenir après son départ

**Succès — à quoi ressemble un projet réussi :**
Pour Marc, le succès signifie livrer la migration dans les délais, avec zéro perte de données critique, sans passer ses nuits à déboguer des mappings. Après avoir utilisé la solution, Marc doit pouvoir dire : **"J'ai mis en place un connecteur fiable en quelques heures au lieu de plusieurs jours, avec un mapping clair que le client comprend et peut maintenir."**

**Motivations :**
- Réduire le temps passé sur des tâches répétitives
- Livrer dans les délais pour maintenir sa crédibilité et celle de l'ESN
- Garantir la fiabilité et la traçabilité des migrations
- Pouvoir expliquer et justifier chaque transformation

**Appréhensions :**
- Solutions "black box" non auditables
- Perte de contrôle sur la logique métier
- Risque contractuel si la solution ne fonctionne pas comme prévu
- Impossibilité de maintenir après son départ

---

#### Persona 2 : Alex — Développeur SaaS B2B

**Nom et contexte :**
- **Nom :** Alex
- **Rôle :** Développeur Backend / Integration Engineer dans une SaaS B2B
- **Contexte :** Alex travaille dans une scale-up SaaS B2B (50-200 employés) qui doit créer et maintenir des intégrations avec les systèmes de ses clients (ERP, CRM, outils métier)
- **Type d'organisation :** Scale-up tech en croissance, besoin de standardiser les intégrations

**Contraintes et contexte :**
- **Besoin de standardisation :** Doit créer plusieurs intégrations similaires (même ERP, différentes instances clients)
- **Besoin de vitesse :** Time-to-market critique pour répondre aux besoins clients rapidement
- **Besoin de maintenance :** Les intégrations doivent être maintenues dans le temps, pas juste créées une fois
- **Sensibilité support :** Les clients comptent sur ces intégrations pour leur business quotidien

**Workflow actuel :**
- Crée chaque intégration manuellement, même si c'est avec le même type d'ERP
- Duplique le code entre projets sans réutilisabilité
- Passe du temps sur le mapping plutôt que sur les fonctionnalités différenciantes

**Succès :**
Après avoir utilisé la solution, Alex doit pouvoir dire : **"Je peux créer une nouvelle intégration en quelques heures au lieu de plusieurs jours, avec une base standardisée que je peux adapter rapidement pour chaque client."**

**Motivations :**
- Standardiser les intégrations pour réduire la dette technique
- Accélérer le time-to-market pour répondre aux besoins clients
- Maintenir la qualité et la fiabilité des intégrations

**Appréhensions :**
- Solutions trop rigides qui ne s'adaptent pas aux cas spécifiques clients
- Manque de contrôle sur les cas edge
- Difficulté à maintenir les intégrations dans le temps

---

#### Persona 3 : Sophie — Tech Lead / Architecte

**Nom et contexte :**
- **Nom :** Sophie
- **Rôle :** Tech Lead / Architecte dans une PME Tech
- **Contexte :** Sophie ne crée pas les connecteurs elle-même, mais doit choisir l'outil d'intégration et s'assurer qu'il répond aux besoins de l'équipe sur le long terme
- **Type d'organisation :** PME Tech (200-1000 employés), avec intégrations critiques

**Contraintes et contexte :**
- **Standardisation :** Veut un outil standard, auditable, reproductible pour toute l'équipe
- **Sensibilité dette technique :** Consciente des risques de solutions qui créent de la dette technique
- **Vision long terme :** Doit penser à la maintenance et à l'évolution dans le temps
- **Validation technique :** Doit valider que l'outil s'intègre bien dans la stack technique existante

**Workflow actuel :**
- Évalue plusieurs solutions d'intégration
- Valide les choix techniques avec l'équipe
- S'assure que les solutions sont maintenables

**Succès :**
Après avoir adopté la solution, Sophie doit pouvoir dire : **"J'ai un outil standard, auditable et maintenable qui réduit la dette technique tout en permettant à l'équipe de livrer rapidement."**

**Motivations :**
- Réduire la dette technique
- Standardiser les pratiques d'intégration
- Garantir la maintenabilité sur le long terme

**Appréhensions :**
- Solutions qui créent de la dette technique
- Vendor lock-in
- Manque de contrôle sur l'évolution future

---

### Secondary Users

#### CTO / Head of Engineering

**Rôle :** Approuve l'outil et valide l'investissement
**Besoins :** Réduire les risques et les coûts cachés, garantir un ROI mesurable
**Influence :** Décision budgétaire et alignement stratégique

#### DevOps / Platform Engineers

**Rôle :** Opèrent les intégrations en production
**Besoins :** Outils observables et déterministes, intégration CI/CD, monitoring et alerting
**Influence :** Validation opérationnelle et support infrastructure

#### QA / Équipes métier

**Rôle :** Testent et valident les migrations et intégrations
**Besoins :** Comprendre les règles appliquées, valider les mappings, tracer les transformations
**Influence :** Validation fonctionnelle et acceptation des migrations

---

### User Journey

#### Journey — Marc (Intégrateur ERP)

**Discovery (Découverte) :**
- Découvre l'outil via une recommandation d'un collègue ou une recherche Google
- Lit la documentation et les exemples pour comprendre l'approche
- Vérifie que l'outil permet le contrôle total et l'audit trail nécessaire

**Onboarding (Intégration) :**
- Setup rapide (< 15 min), installation du runtime portable
- Première intégration : Génération d'un connecteur de test à partir d'OpenAPI
- Découverte du format déclaratif : Comprend comment le mapping est défini et peut être édité

**Core Usage (Utilisation principale) :**
- Génère un pipeline déclaratif pour le projet de migration ERP en cours
- Utilise l'IA assistive pour proposer des mappings, puis valide et ajuste manuellement
- Teste le pipeline avec des données réelles, vérifie la fiabilité
- Utilise les fichiers de configuration générés dans son workflow existant (versioning, CI/CD, etc.)

**Success Moment (Moment de succès) :**
- Réalise que le pipeline déclaratif fonctionne correctement en quelques heures au lieu de plusieurs jours
- Peut expliquer clairement le mapping au client grâce aux configurations de pipeline explicites
- Livre le projet dans les délais avec un audit trail complet

**Long-term (Long terme) :**
- Réutilise l'outil sur d'autres projets similaires
- Partage les templates avec son équipe
- Recommande l'outil à ses collègues et clients

## Success Metrics

### User Success Metrics

#### Persona 1 : Marc (Intégrateur ERP)

**Résultats concrets qui définissent le succès pour Marc :**

Marc considère la solution comme un succès si elle lui permet de :

1. **Réduction drastique du temps de création de connecteur**
   - Avant : 2 à 5 jours pour analyser les APIs, définir le mapping et écrire le connecteur
   - Après : 1 à 4 heures pour un premier connecteur fonctionnel
   - Objectif cible : ≥70% de réduction du temps total

2. **Fiabilité des migrations**
   - Taux de succès des synchronisations/migrations ≥99.9%
   - Zéro perte de données critique (clients, commandes, factures)
   - Capacité à exécuter des dry-runs et validations avant production

3. **Réduction des bugs et du rework**
   - Moins d'itérations correctives après la première exécution
   - Diminution mesurable du temps passé en débogage de mapping et de transformations

4. **Livrable clair et professionnel**
   - Capacité à livrer : un pipeline déclaratif explicite, une documentation automatique du mapping, un artefact compréhensible par le client ou un auditeur
   - Réduction du besoin d'explications manuelles post-mission

5. **Réutilisabilité et capitalisation**
   - Possibilité de réutiliser ou adapter un connecteur existant sur un projet similaire
   - Objectif : réutiliser au moins 30-50% du travail d'un projet à l'autre

6. **Satisfaction client et crédibilité professionnelle**
   - Clients satisfaits, moins de stress en fin de projet
   - Indicateur qualitatif clé : recommandations ou renouvellement de missions

#### Persona 2 : Alex (Développeur SaaS B2B)

**Signes que la solution apporte de la valeur :**

Alex considère la solution comme un succès s'il peut :

1. **Créer une nouvelle intégration client en quelques heures**
   - Délai cible : <1 journée par intégration standard

2. **Standardiser les intégrations**
   - Utilisation d'un format de connecteur commun pour plusieurs clients
   - Réduction de la logique spécifique par client

3. **Réduire la maintenance long terme**
   - Moins de correctifs lors des évolutions d'API
   - Moins de dette technique liée aux intégrations

4. **Accélérer l'onboarding client**
   - Intégrations plus rapides = time-to-value plus court pour ses clients

#### Persona 3 : Sophie (Tech Lead / Architecte)

**Indicateurs de succès pour l'équipe :**

Sophie considère que la solution fonctionne si elle observe :

1. **Réduction de la dette technique liée aux intégrations**
   - Moins de code ad hoc
   - Plus de logique explicite et standardisée

2. **Maintenabilité accrue**
   - Pipelines déclaratifs lisibles, testables, s'intégrant dans les outils existants
   - Moins de dépendance à des connaissances individuelles

3. **Meilleure prévisibilité des projets**
   - Estimation plus fiable des efforts d'intégration
   - Moins de surprises en fin de projet

4. **Adoption naturelle par l'équipe**
   - Les développeurs choisissent l'outil sans contrainte managériale
   - Indicateur clé : usage récurrent sans friction

---

### Business Objectives

#### Objectifs à 3 mois — MVP / Beta

**Objectif principal :** Valider le product-problem fit avec des utilisateurs réels.

**Succès à 3 mois signifie :** "Des intégrateurs et développeurs utilisent réellement l'outil sur des projets concrets et en retirent une valeur immédiate."

**Objectifs quantitatifs :**
- **Utilisateurs actifs :** 50-100 utilisateurs techniques actifs
- **Connecteurs générés :** 100-200 connecteurs créés
- **Cas d'usage réels :** Au moins 10-15 projets d'intégration/migration en conditions réelles

**Engagement initial :**
- ≥50% des utilisateurs génèrent au moins 1 connecteur
- ≥30% reviennent au moins une fois (D30)

**Validation qualitative :**
- NPS ≥35
- Feedback explicite confirmant un gain de temps significatif (jours → heures)

#### Objectifs à 12 mois — Phase de croissance

**Objectif principal :** Atteindre une traction commerciale claire et reproductible.

**Succès à 12 mois signifie :** "Le produit est utilisé régulièrement, payé, et recommandé dans les équipes techniques."

**Objectifs quantitatifs :**
- **Utilisateurs actifs :** 500-1000 utilisateurs
- **Adoption récurrente :** ≥60% des utilisateurs génèrent au moins 1 connecteur par mois
- **Revenue :** $50k-100k ARR (premiers clients payants récurrents : SaaS, ESN, PME Tech)
- **Engagement & rétention :** D30 >30%, D90 >20%

**Positionnement :** Reconnaissance comme outil "developer-first" crédible pour les intégrations API

#### Objectifs à 24 mois — Phase de scale

**Objectif principal :** Devenir une référence sur le segment developer-first des intégrations API.

**Succès à 24 mois signifie :** "Le produit est un standard de facto pour créer et maintenir des connecteurs API côté développeurs."

**Objectifs quantitatifs :**
- **Utilisateurs actifs :** 2000-5000 utilisateurs
- **Revenue :** $500k-1M ARR
- **Expansion revenue :** >30% (upsell équipes, usage accru)

**Positionnement marché :** Référence reconnue sur le segment des intégrations API developer-first, adoption récurrente dans des organisations multi-équipes

---

### Key Performance Indicators

#### KPIs Utilisateurs

**Acquisition :**
- 50-100 nouveaux utilisateurs/mois en phase early
- Croissance MoM utilisateurs : 10-20%
- Acquisition majoritairement organique (communautés dev, recommandations)

**Engagement :**
- ≥60% des utilisateurs génèrent ≥1 connecteur/mois
- 2-3 connecteurs générés/utilisateur/mois

**Retention :**
- D1 >40%
- D30 >25% (MVP), >30% (croissance)
- D90 >15% (croissance), >20% (scale)

**Value realization :**
- Temps moyen pour générer un premier connecteur : <4 heures
- ≥70% de réduction du temps de création de connecteur

#### KPIs Business

**Revenue :**
- ARR année 1 : $50k-100k
- ARR année 2 : $500k-1M
- MRR/ARR comme métriques centrales

**CAC/LTV :**
- CAC <$500
- LTV >$5k
- Ratio LTV/CAC >3:1

**Churn :**
- Churn mensuel <5%

**Expansion :**
- Expansion revenue >30%

**Efficacité :**
- Coûts opérationnels maîtrisés via un runtime unique
- Marges élevées (produit logiciel, peu de support humain par intégration)

#### KPIs Produit

**Adoption :**
- 2-3 connecteurs générés/utilisateur/mois

**Qualité :**
- >95% des connecteurs générés fonctionnels dès la première itération

**Product-market fit :**
- NPS >40 (maturité), ≥35 (MVP)

**Efficacité utilisateur :**
- ≥70% de réduction du temps de création de connecteur

#### KPIs Stratégiques

**Positionnement :**
- Reconnaissance comme solution leader developer-first sur le segment ciblé

**Référence :**
- >30% des utilisateurs recommandent activement le produit

**Différenciation :**
- >50% des utilisateurs préfèrent la solution à un iPaaS traditionnel ou à du développement ad hoc

## MVP Scope

### Core Features

#### 1. Ingestion de deux OpenAPI (source + cible)

**Indispensable.**  
Import de spécifications OpenAPI REST (JSON/YAML)  
Extraction des endpoints, schémas et types  
Support d'un cas simple mais réel : REST, JSON, Auth standard (API key / OAuth2 basique)

**Justification :** Sans cela, Marc n'économise aucun temps.

#### 2. Génération d'un pipeline déclaratif initial

**Cœur du MVP.**  
Génération automatique d'une configuration de pipeline déclaratif modulaire (Input → Filter → Output) avec :
- Étapes Input (source) et Output (cible)
- Étapes Filter pour mapping champ à champ et transformations
- Structure explicite (lisible, diffable)

Le pipeline est modifiable manuellement dès le départ.

**Justification :** C'est ce qui remplace les jours de "plumbing" initial.

#### 3. IA assistive pour le mapping (suggestion uniquement)

**Essentielle, mais strictement assistive.**  
Proposer des correspondances probables :
- customer_id → client_id
- dates, enums, montants

Afficher une confidence  
Aucune décision automatique non validée

**Justification :** Sans IA : gain partiel. Avec IA : passage de "rapide" à "radicalement plus rapide".

#### 4. Runtime portable minimal (CLI)

**Indispensable pour rendre le pipeline réel.**  
Un runtime exécutable via CLI : `connector run my-pipeline.yaml`

Capable de :
- Exécuter les étapes Input (pull données source)
- Exécuter les étapes Filter (appliquer mapping + transformations)
- Exécuter les étapes Output (push vers la cible)

Logs clairs + erreurs explicites

**Justification :** Sans runtime, le pipeline n'est qu'un document.

#### 5. Mode test / dry-run

**Critique pour la confiance.**  
Exécution sans effet côté cible  
Validation du mapping et des transformations  
Détection d'erreurs avant prod

**Justification :** Sans cela, Marc ne l'utilisera pas sur un projet réel.

#### 6. Documentation automatique du mapping

**Essentielle pour la livraison client.**  
Génération d'un document lisible :
- Champs source → champs cible
- Transformations appliquées

Utilisable pour : validation client, audit, passation

**Justification :** Forte valeur perçue, coût technique faible.

---

### Out of Scope for MVP

#### UI graphique avancée

**Pas d'éditeur visuel complexe, pas de drag & drop**  
CLI + fichiers suffisent pour le MVP

#### Support multi-protocoles

**Pas de SOAP, pas de XML, pas de formats exotiques**  
REST + JSON uniquement

#### Orchestration complexe

**Pas de workflows BPM, pas de multi-step conditionnel avancé**  
1 source → 1 cible uniquement

#### Runtime managé obligatoire

**Pas de plateforme SaaS lourde, pas d'exécution cloud imposée**  
CLI locale / CI uniquement

#### Auto-adaptation en production

**L'IA ne modifie rien en runtime, pas de self-healing automatique**  
Toutes les modifications sont explicites et validées par le développeur

---

### MVP Success Criteria

**Le MVP est considéré comme réussi si :**
- 50-100 utilisateurs beta actifs
- ≥70% déclarent un gain de temps significatif
- NPS ≥35
- Cas réels de migration/intégration livrés

**Marc peut dire :** "J'ai livré un connecteur fiable en quelques heures, pas en plusieurs jours."

#### Point de décision post-MVP

**On passe au-delà du MVP si :**
- Les utilisateurs reviennent spontanément
- Ils demandent des fonctionnalités supplémentaires (pas juste "ça ne marche pas")
- Ils sont prêts à payer ou à s'engager plus fortement

---

### Future Vision

**Si le MVP fonctionne, vision 2-3 ans :**

**UI avancée de mapping :** Éditeur visuel pour faciliter la création et la modification de connecteurs

**Librairies runtime par langage :** Runtime disponible en Go, Rust, Node.js, Python, etc.

**Support de cas complexes :** Multi-sources, règles conditionnelles, transformations avancées

**Exécution managée optionnelle :** Plateforme SaaS optionnelle pour exécution cloud, monitoring, etc.

**Bibliothèque de connecteurs réutilisables :** Marketplace de connecteurs pré-configurés pour APIs populaires (Salesforce, SAP, etc.)

**Note :** Rien de tout cela n'est requis pour prouver la valeur dans le MVP.

---

<!-- Content will be appended sequentially through collaborative workflow steps -->
