---
stepsCompleted: [1, 2, 3, 4, 6, 7, 8, 9, 10, 11]
inputDocuments: 
  - 'product-brief-cannectors-2026-01-10.md'
  - 'research/market-api-connector-automation-saas-research-2026-01-10.md'
briefCount: 1
researchCount: 1
brainstormingCount: 0
projectDocsCount: 0
workflowType: 'prd'
lastStep: 11
---

# Product Requirements Document - Cannectors

**Author:** Cano
**Date:** 2026-01-10

## Executive Summary

Plateforme SaaS B2B developer-first qui automatise la génération de connecteurs déclaratifs modulaires entre systèmes métiers à partir de spécifications OpenAPI, avec IA assistive pour le mapping des données.

**Architecture technique :** Génération de pipelines déclaratifs modulaires (Input → Filter → Output) interprétés par un runtime portable et déterministe, avec IA utilisée uniquement pour assister la génération (pas l'exécution), laissant au développeur un contrôle total sur la logique métier, la sécurité et l'exécution.

**Modèle de connecteur :** Chaque connecteur est un pipeline déclaratif composé de modules explicites, versionnables et interprétés par le runtime. Les modules Input (HTTP, Webhook, SQL, Pub/Sub), Filter (mapping, conditions, transformations) et Output (HTTP, Webhook, SQL, Pub/Sub) sont assemblés de manière déclarative, inspiré du pattern Logstash mais orienté connecteurs API developer-first.

**Objectif :** Réduire le temps de création de connecteurs de 2-5 jours à 1-4 heures pour un connecteur fonctionnel, et moins d'une journée pour un connecteur prêt pour la production.

**Positionnement :** Runtime universel de connecteurs API, OpenAPI-first, orienté backend / integration engineers. Ce n'est pas un iPaaS générique, ni un ETL data-centric, mais un runtime déclaratif de connecteurs avec génération assistée par IA. Automatiser la partie la plus pénible (analyse des schémas OpenAPI, génération des déclarations de mapping et transformations) tout en conservant un runtime déterministe, explicable et maîtrisé par les développeurs.

### What Makes This Special

La différenciation clé repose sur un **runtime déclaratif de connecteurs API modulaire** exécuté par un **runtime déterministe**, avec une **IA uniquement assistive**, ce qui permet **vitesse, contrôle, auditabilité et maintenabilité** là où les solutions existantes imposent du no-code ou du code généré fragile.

**Positionnement clarifié :**
- **Ce n'est pas un iPaaS générique** : Pas d'orchestration workflow complexe, pas de no-code, focus sur connecteurs API déclaratifs
- **Ce n'est pas un ETL data-centric** : Pas de focus sur transformation de données massives, mais sur intégrations API backend-to-backend
- **C'est un runtime de connecteurs API** : OpenAPI-first, orienté backend / integration engineers, avec génération assistée par IA

Cette approche combine :
- **Pipeline modulaire déclaratif** : Connecteurs composés de modules Input/Filter/Output explicites, versionnables, interprétés par le runtime (pas de code généré)
- **Automatisation complète** : OpenAPI → pipeline déclaratif production-ready (auth, pagination, retry, mapping, gestion erreurs)
- **IA assistive pour la génération uniquement** : Suggestions de mapping et détection de patterns lors de la création, pas d'exécution non déterministe
- **Runtime portable et déterministe** : Runtime unique, versionné et maintenu par la plateforme, garantissant exécution prévisible et contrôlable
- **Contrôle développeur total** : Déclarations explicites, lisibles, versionnables, éditable et testable par le développeur
- **Maintenabilité supérieure** : Format déclaratif stable vs code généré avec dépendances fragiles, runtime unique à maintenir vs dépendances multiples

Contrairement aux iPaaS traditionnels (no-code, vendor lock-in) et aux outils developer-first existants (code généré fragile, mapping manuel), cette solution offre le meilleur des deux mondes : automatisation complète avec contrôle total, via un modèle de pipeline modulaire inspiré de Logstash mais orienté connecteurs API developer-first.

## Technical Architecture

### Connector Model: Declarative Modular Pipeline

**Concept fondamental :** Un connecteur n'est plus uniquement un "mapping OpenAPI → OpenAPI", mais un **pipeline déclaratif modulaire** inspiré du pattern Input → Filter → Output, comparable conceptuellement à Logstash, mais orienté connecteurs API developer-first.

**Architecture du pipeline :**

Chaque connecteur est défini par une configuration déclarative composée de modules explicites, versionnables et interprétés par le runtime :

#### Input Modules (Sources de données)

**MVP :**
- **HTTP Request (Polling + CRON)** : Récupération périodique de données depuis une API REST via polling avec planification CRON
- **Webhook** : Réception de données en temps réel via webhooks HTTP

**Post-MVP :**
- **SQL Query (Polling + CRON)** : Récupération de données depuis bases de données relationnelles via requêtes SQL planifiées
- **Pub/Sub / Kafka** : Intégration avec systèmes de messagerie asynchrone (Kafka, RabbitMQ, etc.)

#### Filter Modules (Transformation et logique)

**MVP :**
- **Mapping de données déclaratif (OpenAPI-driven)** : Mapping champ-à-champ entre schémas source et cible, généré automatiquement depuis OpenAPI
- **Conditions simples (if/else)** : Logique conditionnelle basique pour routing et filtrage de données

**Post-MVP :**
- **Transformations de données avancées** : Transformations complexes (formatting, calculs, agrégations)
- **Cloning / Fan-out** : Duplication de données vers plusieurs sorties
- **Requêtes externes dépendantes** : Appels API conditionnels basés sur valeurs d'entrée
- **Scripting avancé** : Exécution de scripts personnalisés pour logique métier complexe (hors MVP)

#### Output Modules (Destinations)

**MVP :**
- **HTTP Request** : Envoi de données vers APIs REST via requêtes HTTP

**Post-MVP :**
- **Webhook** : Déclenchement de webhooks externes
- **SQL** : Écriture de données dans bases de données relationnelles
- **Pub/Sub / Kafka** : Publication de données vers systèmes de messagerie asynchrone

### Runtime Architecture

**Caractéristiques du runtime :**

1. **Interprétation déclarative** : Le runtime interprète les déclarations de modules, pas de code généré
2. **Déterminisme** : Exécution 100% prévisible et reproductible
3. **Portabilité** : Runtime unique, versionné, portable (Go/Rust), exécutable localement ou en CI/CD
4. **Modularité** : Chaque module est indépendant, composable, et versionnable
5. **Explicité** : Toute la logique est visible dans les déclarations, pas de "magie" cachée

**CLI comme ingesteur de configuration :**

Le CLI est l'ingesteur de configuration qui définit le format exact en pratique. Il :
- Lit et valide les fichiers de configuration déclaratifs (YAML/JSON)
- Exécute les pipelines modulaires selon les déclarations
- Valide la structure et la sémantique des configurations
- Gère les erreurs et génère des logs explicites

**Ordre de développement :** 
1. Le format de configuration est défini en premier
2. Le CLI (ingesteur) est développé et doit être fonctionnel avec des configurations générées manuellement avant de passer au Front
3. Le Front (générateur) est développé ensuite pour générer automatiquement des configurations depuis OpenAPI avec mapping IA assistive, selon le format validé par le CLI

**Exemple de structure déclarative (conceptuel) :**

```yaml
connector:
  name: "erp-migration"
  version: "1.0.0"
  
  input:
    type: "http_polling"
    source_api: "old-erp-api"
    schedule: "0 */6 * * *"  # CRON
    endpoint: "/api/customers"
    
  filter:
    - type: "mapping"
      source_schema: "old-erp-customer"
      target_schema: "new-erp-client"
      mappings:
        - source: "customer_id"
          target: "client_id"
          confidence: 0.95
        - source: "name"
          target: "full_name"
    - type: "condition"
      if: "status == 'active'"
      then: "process"
      else: "skip"
      
  output:
    type: "http_request"
    target_api: "new-erp-api"
    endpoint: "/api/clients"
    method: "POST"
```

### Key Architectural Principles

1. **Déclaratif, pas génératif** : Les connecteurs sont des déclarations interprétées, pas du code généré
2. **Modulaire et composable** : Modules réutilisables assemblés en pipelines
3. **OpenAPI-first** : Génération automatique depuis spécifications OpenAPI
4. **IA uniquement assistive** : IA pour suggestions de mapping, jamais pour exécution
5. **Runtime unique** : Un seul runtime à maintenir, évolutif indépendamment des déclarations
6. **Versionnable** : Format déclaratif diffable, auditable, compatible avec systèmes de versioning standards

## Project Classification

**Technical Type:** saas_b2b
**Domain:** general
**Complexity:** low
**Project Context:** Greenfield - new project

**Classification Details:**

- **Project Type:** SaaS B2B platform orientée développeurs (détection: SaaS, B2B, platform, developer-first, integrations, API)
- **Domain:** Outils développeurs généralistes (pas de domaine réglementaire spécifique comme healthcare, fintech)
- **Complexity Level:** Faible - pratiques standards de développement logiciel, pas de contraintes réglementaires ou techniques complexes
- **Context:** Projet greenfield nécessitant définition complète de la vision produit

## Success Criteria

### User Success

**Critères de succès utilisateur :**

Les utilisateurs techniques expérimentés considèrent le produit comme un succès quand ils peuvent :
- **Réduire drastiquement le temps de création de connecteur** : ≥70% de réduction (de 2-5 jours à 1-4 heures pour un connecteur fonctionnel)
- **Livrer avec fiabilité** : ≥99.9% de succès des synchronisations/migrations, zéro perte de données critique
- **Réduire le rework** : Moins d'itérations correctives, moins de débogage de mapping et transformations
- **Livrer un mapping clair** : Déclarations explicites, documentation automatique du mapping, artefact compréhensible et transmissible
- **Réutiliser effectivement** : 30-50% du travail réutilisable entre projets similaires
- **Adopter récurrent** : Utilisation régulière sans friction, intégration naturelle dans le workflow

**Métriques utilisateur clés :**
- Temps moyen pour générer un premier connecteur : <4 heures
- NPS ≥35 (MVP), >40 (maturité)
- ≥70% de réduction du temps de création de connecteur
- >95% des connecteurs générés fonctionnels dès la première itération

**En une phrase :** Quand l'utilisateur dira « ça en valait la peine » : Quand il pourra livrer une intégration critique plus vite, avec moins de stress, moins de débogage, un mapping clair et un livrable qu'il est fier de transmettre.

### Business Success

**Objectifs business :**

**3 mois (MVP/Beta) - Validation product-problem fit :**
- 50-100 utilisateurs techniques actifs
- 100-200 connecteurs créés
- ≥50% des utilisateurs génèrent au moins 1 connecteur
- ≥30% reviennent au moins une fois (D30)
- NPS ≥35, feedback confirmant gain de temps significatif

**12 mois (Croissance) - Traction commerciale :**
- 500-1000 utilisateurs actifs
- $50k-100k ARR (premiers clients payants récurrents : SaaS, ESN, PME Tech)
- ≥60% des utilisateurs génèrent au moins 1 connecteur par mois
- D30 >30%, D90 >20%
- Reconnaissance comme outil "developer-first" crédible

**24 mois (Scale) - Référence marché :**
- 2000-5000 utilisateurs actifs
- $500k-1M ARR
- Expansion revenue >30% (upsell équipes, usage accru)
- Positionnement : Référence reconnue sur le segment developer-first

**KPIs business :**
- ARR : $50k-100k (année 1), $500k-1M (année 2)
- CAC <$500, LTV >$5k, ratio LTV/CAC >3:1
- Churn mensuel <5%
- Expansion revenue >30%
- Adoption en équipe : Mesurer adoption par équipes entières vs individus isolés
- Reconnaissance developer-first : Spontanément recommandé comme référence
- Croissance principalement organique : Acquisition majoritairement organique (communautés dev, recommandations)

**En une phrase :** Comment savoir que "ça marche" business : Quand le produit est adopté par des équipes entières, génère des revenus récurrents avec un CAC faible, et est spontanément recommandé par les développeurs comme référence pour les intégrations API.

### Technical Success

**Critères de succès technique :**

**Fiabilité et stabilité :**
- Runtime déterministe et stable : Exécution prévisible, pas de comportements aléatoires
- >95% des connecteurs générés fonctionnels dès la première itération
- Gestion d'erreurs robuste et explicite
- Pas de dépendances externes fragiles dans les déclarations générées

**Maintenabilité :**
- Connecteurs lisibles et maintenables dans le temps : Format déclaratif stable, backward compatible
- Déclarations explicites, versionnables, testables
- Runtime unique à maintenir (pas de dépendances frameworks externes)
- Évolution indépendante runtime vs déclarations possible

**Intégration workflow développeur :**
- Intégration naturelle dans les workflows existants : CI/CD, CLI moderne
- Pas d'adaptation nécessaire de la façon de travailler des développeurs
- Plugins VSCode/CI-CD pour intégration native
- Déclarations versionnables (format texte, diffable, compatible avec systèmes de versioning standards)

**En une phrase :** Ça marche techniquement quand le runtime est déterministe et stable, que les connecteurs restent lisibles et maintenables dans le temps, et que les développeurs l'intègrent naturellement dans leurs workflows sans adapter leur façon de travailler.

### Measurable Outcomes

**Métriques mesurables consolidées :**

**Utilisateur :**
- Temps moyen pour générer un premier connecteur : <4 heures
- ≥70% de réduction du temps de création de connecteur
- NPS ≥35 (MVP), >40 (maturité)
- >95% des connecteurs générés fonctionnels dès la première itération
- ≥60% des utilisateurs génèrent ≥1 connecteur/mois

**Business :**
- ARR : $50k-100k (année 1), $500k-1M (année 2)
- CAC <$500, LTV >$5k, LTV/CAC >3:1
- Churn mensuel <5%
- Expansion revenue >30%
- Croissance MoM utilisateurs : 10-20% (early stage)

**Technique :**
- >95% des connecteurs générés fonctionnels dès la première itération
- Runtime déterministe : 100% des exécutions prévisibles (pas de comportements aléatoires)
- Backward compatibility : Format déclaratif stable entre versions

**Adoption :**
- Adoption en équipe : Mesure adoption par équipes entières vs individus isolés
- Recommandations organiques : >30% des utilisateurs recommandent activement
- Intégration workflow : Usage récurrent sans friction, intégration CI/CD standard

## User Journeys

### Journey 1: Marc — From Deadline Panic to Confident Delivery

Marc est un consultant ERP sénior chez une ESN spécialisée. Un vendredi après-midi, il apprend qu'il doit livrer une migration ERP critique dans 12 jours pour un client important. Il a déjà passé 3 jours sur un projet précédent à analyser manuellement des OpenAPI incomplètes et à mapper des données via Excel. Il est inquiet : les délais sont serrés, les données sont critiques (clients, commandes, factures), et il doit pouvoir justifier chaque transformation auprès du client et d'un auditeur potentiel.

Lundi matin, un collègue lui recommande l'outil après l'avoir utilisé avec succès. Marc lit la documentation en 20 minutes et décide de tester sur un cas simple. En 15 minutes, il installe le runtime portable via CLI. Il importe les deux OpenAPI (ancien ERP et nouveau ERP SaaS), et l'outil génère automatiquement un connecteur déclaratif initial. Marc examine le fichier YAML généré : c'est lisible, le mapping est explicite, et il peut tout modifier. L'IA assistive a proposé des correspondances probables (customer_id → client_id) avec un niveau de confiance. Marc valide les mappings critiques et ajuste quelques cas edge manuellement.

Mardi, il exécute un dry-run sur un échantillon de données. Le runtime exécute le connecteur de façon déterministe, avec des logs clairs. Il détecte deux erreurs de mapping qu'il corrige rapidement. Mercredi, il ajoute les déclarations de connecteur dans l'inventaire de production du projet client (déjà versionné). Jeudi, il génère automatiquement la documentation du mapping : un document lisible que le client peut comprendre. Vendredi, il exécute la migration réelle avec confiance. Elle se termine sans erreur critique.

**Le moment de succès :** Marc livre la migration dans les délais, avec un audit trail complet. Le client peut comprendre chaque transformation grâce aux déclarations explicites. Marc n'a pas passé ses nuits à déboguer. Six mois plus tard, il réutilise l'outil sur un projet similaire, réutilise 40% de son travail précédent, et recommande l'outil à d'autres consultants de son ESN.

### Journey 2: Alex — From Repetitive Work to Standardized Excellence

Alex travaille dans une scale-up SaaS B2B qui se développe rapidement. Son équipe doit créer des intégrations avec les systèmes clients (ERP, CRM) pour chaque nouveau client. Le problème : même si c'est le même type d'ERP (SAP), chaque intégration est créée manuellement, le code est dupliqué, et la maintenance est pénible. Alex passe trop de temps sur le mapping plutôt que sur des fonctionnalités différenciantes qui font la valeur du produit.

Un matin, son CTO lui suggère d'évaluer un nouvel outil d'intégration qui automatise la génération de connecteurs. Alex teste l'outil sur une intégration réelle avec un client pilote. Il importe les OpenAPI des deux systèmes et génère un connecteur déclaratif en moins d'une heure. Le format déclaratif est standardisé : même structure pour toutes les intégrations. Alex adapte rapidement le mapping pour les spécificités du client, puis teste avec un dry-run.

La première intégration cliente fonctionne. Alex crée un template de base pour les intégrations SAP, qu'il réutilise pour les clients suivants. Il adapte le template à chaque cas, ce qui prend 2-3 heures au lieu de 3-4 jours. L'équipe standardise rapidement les intégrations principales.

**Le moment de succès :** Alex crée une nouvelle intégration client en 4 heures au lieu de 5 jours. Il a une base standardisée qu'il peut adapter rapidement. La maintenance est simplifiée : si une API change, il met à jour la déclaration, pas du code complexe. Trois mois plus tard, l'équipe a standardisé 80% des intégrations, réduit la dette technique, et peut répondre plus rapidement aux besoins clients.

### Journey 3: Sophie — From Tool Evaluation to Team Adoption

Sophie est Tech Lead dans une PME Tech qui doit choisir un outil d'intégration pour standardiser les pratiques de l'équipe. Elle a déjà évalué plusieurs solutions : certaines sont trop lourdes (iPaaS Enterprise), d'autres génèrent du code fragile qui devient difficile à maintenir. Elle cherche un outil standard, auditable, maintenable, qui s'intègre bien avec la stack technique existante (CI/CD).

Elle découvre l'outil via un article technique sur les outils developer-first. Elle teste d'abord avec un cas réel : une intégration interne entre deux systèmes. En une journée, elle génère un connecteur, le teste, et l'ajoute à l'inventaire de production de l'équipe. Elle apprécie que les déclarations soient lisibles, versionnables (format texte, diffable), et que le runtime soit déterministe.

Sophie fait une présentation à l'équipe : elle montre les déclarations, explique l'approche déclarative, et démontre l'intégration avec CI/CD. L'équipe est convaincue : le format est clair, pas de vendor lock-in, contrôle total. Sophie met en place un template d'équipe et documente les bonnes pratiques.

**Le moment de succès :** Sophie a un outil standard, auditable et maintenable qui réduit la dette technique tout en permettant à l'équipe de livrer rapidement. Les développeurs adoptent l'outil naturellement, sans contrainte managériale. Six mois plus tard, l'équipe a standardisé toutes ses intégrations, la maintenance est simplifiée, et les estimations sont plus fiables.

### Journey Requirements Summary

Ces parcours révèlent les exigences fonctionnelles suivantes :

**Onboarding et découverte :**
- Documentation claire et exemples accessibles
- Setup rapide : installation runtime portable <15 min
- Preuve de concept facile : première intégration en <1 heure

**Génération et édition :**
- Ingestion OpenAPI : Import de deux spécifications (source + cible)
- Génération automatique : OpenAPI → connecteur déclaratif initial
- IA assistive : Suggestions de mapping avec niveau de confiance
- Édition manuelle : Contrôle total sur les déclarations (format lisible, diffable)

**Testing et validation :**
- Mode dry-run : Exécution sans effet côté cible pour validation
- Logs clairs : Traçabilité et debugging facilités
- Détection d'erreurs : Identification précoce des problèmes de mapping

**Versioning et collaboration :**
- Déclarations versionnables : Format texte, diffable, compatible avec systèmes de versioning standards (développeurs peuvent ajouter les fichiers dans leur inventaire de production déjà versionné)
- Documentation automatique : Génération de docs lisibles pour clients/auditeurs

**Exécution :**
- Runtime déterministe : Exécution prévisible et contrôlable
- CLI portable : Exécution locale ou CI/CD
- Gestion d'erreurs : Traitement robuste avec messages explicites

**Réutilisabilité :**
- Templates : Réutilisation de connecteurs existants
- Standardisation : Format commun pour équipes

Ces exigences sont alignées avec le scope MVP et soutiennent les critères de succès définis pour les utilisateurs, le business et la technique.

## Innovation & Novel Patterns

### Detected Innovation Areas

L'innovation du produit repose moins sur une technologie isolée que sur une **combinaison cohérente** de quatre éléments fondamentaux :

1. **Pipeline déclaratif modulaire** : Connecteurs composés de modules Input/Filter/Output explicites, versionnables, interprétés par le runtime (pas de code généré), inspiré du pattern Logstash mais orienté connecteurs API developer-first

2. **Connecteurs déclaratifs explicites** : Format DSL/configuration explicite (YAML/JSON/Toml) définissant les modules et leur configuration, lisible, versionnable et éditable par le développeur

3. **IA uniquement assistive** : IA utilisée uniquement pour assister la génération des déclarations (suggestions de mapping, détection de patterns), pas pour l'exécution, garantissant un runtime déterministe et contrôlable

4. **Runtime portable déterministe** : Runtime unique interprétant les déclarations modulaires, portable (Go/Rust), déterministe, versionné et maintenu par la plateforme

**Innovation de combinaison :** Cette combinaison permet **vitesse, contrôle et maintenabilité** là où les solutions existantes imposent soit du no-code (vendor lock-in, manque de contrôle) soit du code généré fragile (dépendances multiples, maintenance complexe). Le modèle de pipeline modulaire offre flexibilité et composabilité tout en restant déclaratif et contrôlable.

**Ce qui n'existe pas aujourd'hui :** Aucune solution ne combine automatisation complète (OpenAPI → pipeline déclaratif modulaire production-ready), mapping intelligent avec IA assistive, et runtime portable avec contrôle total du développeur via un modèle de modules composables.

### Market Context & Competitive Landscape

**Contexte marché :**

Le marché des outils d'intégration API est fragmenté entre :
- **iPaaS traditionnels** (MuleSoft, Workato, Zapier) : No-code, vendor lock-in, peu orientés développeurs
- **Outils developer-first** (OpenAPI Generator, Prismatic) : Génération partielle, mapping manuel, pas de runtime déterministe
- **Solutions internes** : Code dupliqué, maintenance complexe, pas de standardisation

**Angle mort identifié :** L'innovation réside dans la combinaison unique qui résout le dilemme "automatisation vs contrôle" : automatisation complète avec contrôle total du développeur, grâce à l'approche déclarative avec IA assistive uniquement pour la génération.

**Positionnement innovant :** Offrir le meilleur des deux mondes : la vitesse et l'automatisation des iPaaS, avec le contrôle et la maintenabilité des outils developer-first, via une approche déclarative interprétée par un runtime portable.

### Validation Approach

**Approche de validation pour les aspects innovants :**

1. **Validation de la combinaison cohérente :**
   - MVP : Prouver que la génération automatique + IA assistive + runtime déterministe fonctionne ensemble
   - Métriques : Temps de génération <4h, connecteurs fonctionnels dès première itération >95%, satisfaction développeur (NPS ≥35)

2. **Validation de l'IA uniquement assistive :**
   - Mesurer l'utilité des suggestions de mapping (taux d'acceptation, réduction du temps de mapping)
   - Valider que l'exécution reste déterministe (100% des exécutions prévisibles, pas de comportements aléatoires)
   - Feedback utilisateur : Les développeurs apprécient-ils le contrôle total sur l'exécution ?

3. **Validation du runtime portable déterministe :**
   - Tester la portabilité : on-premise, cloud, CI/CD
   - Valider la déterministe : Exécutions reproductibles, logs explicites
   - Mesurer la maintenabilité : Facilité de mise à jour runtime vs déclarations

4. **Validation de la différenciation :**
   - Comparaison avec iPaaS (no-code) : Les développeurs préfèrent-ils le contrôle déclaratif ?
   - Comparaison avec outils developer-first : Les développeurs apprécient-ils l'automatisation complète ?
   - Métriques : >50% des utilisateurs préfèrent la solution à un iPaaS traditionnel ou à du développement ad hoc

### Risk Mitigation

**Risques liés aux aspects innovants :**

1. **Risque de complexité perçue :**
   - **Risque** : Les développeurs peuvent percevoir l'approche déclarative comme complexe
   - **Mitigation** : Documentation claire, exemples concrets, onboarding rapide (<15 min)
   - **Fallback** : Si complexité trop élevée, simplifier le format déclaratif ou améliorer la génération automatique

2. **Risque d'IA assistive insuffisante :**
   - **Risque** : L'IA ne propose pas de suggestions utiles, réduisant la valeur
   - **Mitigation** : Itérer sur les modèles d'IA, améliorer la détection de patterns, feedback utilisateur continu
   - **Fallback** : Même sans IA assistive efficace, la génération automatique de base reste utile

3. **Risque de runtime portable :**
   - **Risque** : Le runtime peut ne pas être suffisamment portable ou performant
   - **Mitigation** : Tests rigoureux sur différentes plateformes, optimisation continue
   - **Fallback** : Si portabilité problématique, adapter le runtime ou proposer des variantes par plateforme

4. **Risque de non-adoption :**
   - **Risque** : Les développeurs peuvent préférer les solutions existantes
   - **Mitigation** : Démonstration claire de la valeur (gain de temps mesurable), intégration workflow développeur, freemium généreux
   - **Fallback** : Si adoption limitée, pivoter vers un positionnement différent (embedded iPaaS, outils internes)

5. **Risque de compétition :**
   - **Risque** : Les iPaaS établis peuvent ajouter des fonctionnalités similaires
   - **Mitigation** : Vitesse d'exécution, focus développeur, meilleure UX dev, avantage first-mover
   - **Fallback** : Pivoter vers embedded iPaaS ou acquisition par un acteur établi

## SaaS B2B Specific Requirements

### Project-Type Overview

Plateforme SaaS B2B developer-first avec modèle multi-tenant orienté organisation, permissions simplifiées pour adoption rapide, et intégrations prioritaires dans les workflows développeur existants.

### Technical Architecture Considerations

**Architecture multi-tenant :**
- Modèle multi-tenant simple orienté organisation
- Chaque organisation = tenant logique
- Isolation logique stricte des données par organisation
- Pas d'infrastructure dédiée par tenant au MVP (isolation logique uniquement)
- Architecture évolutive pour supporter isolation physique future si nécessaire

**Considérations techniques :**
- Base de données : Isolation logique via organisation_id sur toutes les tables
- API : Toutes les requêtes scoped par organisation
- Sécurité : Validation stricte de l'appartenance organisation à chaque requête
- Performance : Indexation optimisée sur organisation_id pour isolation efficace

### Tenant Model

**Modèle multi-tenant MVP :**

- **Unité de tenancy :** Organisation (chaque organisation = un tenant)
- **Isolation :** Isolation logique stricte des données par organisation
  - Toutes les données (connecteurs, mappings, exécutions) isolées par organisation_id
  - Aucune fuite de données entre organisations possible
  - Validation systématique de l'appartenance organisation à chaque opération
- **Infrastructure :** Isolation logique uniquement au MVP (pas d'infrastructure dédiée par tenant)
- **Évolutivité :** Architecture prête pour isolation physique future si nécessaire (sharding par organisation)

**Gestion des organisations :**
- Création d'organisation lors de l'inscription
- Un utilisateur peut appartenir à plusieurs organisations (avec rôle par organisation)
- Switch entre organisations pour utilisateurs multi-organisations

### RBAC Matrix

**Modèle de permissions MVP (simplifié) :**

**Rôles disponibles :**
- **Owner** : Accès complet à l'organisation
  - Gérer les membres de l'organisation
  - Gérer les paramètres de l'organisation
  - Accès complet aux connecteurs et exécutions
  - Gérer l'abonnement
- **Member** : Accès standard
  - Créer et modifier des connecteurs
  - Exécuter des connecteurs
  - Voir les exécutions et logs
  - Pas d'accès aux paramètres organisation ni gestion membres

**Décisions MVP :**
- Pas de RBAC avancé (pas de permissions fines par ressource)
- Pas de rôles personnalisés
- Modèle simple pour favoriser adoption rapide et limiter complexité
- Évolutivité : Architecture prête pour RBAC avancé futur si nécessaire

**Matrice de permissions :**

| Action | Owner | Member |
|--------|-------|--------|
| Créer connecteur | ✅ | ✅ |
| Modifier connecteur | ✅ | ✅ |
| Exécuter connecteur | ✅ | ✅ |
| Voir exécutions/logs | ✅ | ✅ |
| Gérer membres org | ✅ | ❌ |
| Paramètres organisation | ✅ | ❌ |
| Gérer abonnement | ✅ | ❌ |

### Subscription Tiers

**Modèle Free + Paid :**

**Plan Free (Découverte) :**
- Objectif : Permettre découverte et évaluation du produit
- Limites d'usage :
  - Nombre limité de connecteurs générés par mois (ex: 5-10)
  - Usage individuel (pas d'équipe)
  - Support communautaire uniquement
- Fonctionnalités : Accès complet aux fonctionnalités core (génération, runtime, IA assistive)

**Plan Paid (Production) :**
- Objectif : Usage en production et en équipe
- Limites levées :
  - Connecteurs illimités
  - Usage en équipe (multi-membres)
  - Support prioritaire
- Fonctionnalités : Accès complet + fonctionnalités équipe (gestion membres, collaboration)

**Stratégie pricing MVP :**
- Free tier généreux pour adoption et viralité
- Paid tier accessible pour validation de valeur
- Pricing usage-based ou fixe selon validation marché (à définir)

### Integration List

**Intégrations prioritaires MVP :**

1. **CLI (Command Line Interface)**
   - CLI moderne pour intégration naturelle dans workflows existants
   - Exécution locale (runtime portable)
   - Intégration CI/CD native
   - Scripts et automation supportés

**Intégrations hors scope MVP (volontairement) :**
- Plugins IDE (VSCode, IntelliJ, etc.) → Post-MVP
- Intégrations CI/CD avancées (webhooks, triggers) → Post-MVP
- Intégrations tierces (Slack, notifications) → Post-MVP

**Justification :**
- CLI couvre les besoins workflow développeur essentiels
- Les déclarations sont des fichiers texte (YAML/JSON) que les développeurs peuvent ajouter dans leur inventaire de production déjà versionné
- Limitation volontaire pour MVP focus et vitesse d'itération
- Plugins et intégrations avancées = features de croissance post-MVP

### Compliance Requirements

**Compliance MVP (minimal) :**

**MVP :**
- Conformité GDPR de base (données personnelles, droit à l'effacement)
- Sécurité de base (HTTPS, authentification sécurisée, isolation données)
- Pas de SOC2, ISO27001, ou certifications Enterprise au MVP

**Post-MVP (croissance) :**
- SOC2 Type II pour clients Enterprise
- ISO27001 si nécessaire pour certains marchés
- Certifications sectorielles si expansion verticale

**Justification :**
- MVP focus sur validation product-market fit
- Compliance Enterprise = barrière à l'entrée pour MVP
- Architecture prête pour compliance future (isolation données, audit logs)

### Implementation Considerations

**Considérations d'implémentation SaaS B2B :**

**Multi-tenancy :**
- Middleware d'authentification et scoping organisation systématique
- Tests d'isolation rigoureux (pas de fuite de données entre organisations)
- Monitoring et alerting sur tentatives d'accès cross-tenant

**Permissions :**
- Middleware de vérification de rôle simple mais efficace
- Architecture extensible pour RBAC avancé futur
- UI adaptative selon rôle utilisateur

**Subscription :**
- Système de billing et gestion d'abonnements
- Limitation d'usage selon plan (rate limiting, quotas)
- Upgrade/downgrade de plan fluide

**Intégrations :**
- CLI cross-platform (Windows, Mac, Linux)
- Documentation complète pour intégration CI/CD
- Déclarations en format texte (YAML/JSON) compatibles avec systèmes de versioning standards

## Functional Requirements

### Connector Management

- FR1: Developers can create a new connector pipeline from two OpenAPI specifications (source and target)
- FR2: Developers can view a list of all connectors in their organization
- FR3: Developers can view details of a specific connector including its pipeline configuration (Input/Filter/Output modules)
- FR4: Developers can edit connector declarations (modules, mappings, transformations, endpoint configurations)
- FR5: Developers can delete a connector
- FR6: Developers can duplicate an existing connector as a template
- FR7: Developers can view connector version history (via système de versioning externe utilisé par l'équipe)
- FR8: Developers can compose connectors using Input, Filter, and Output modules
- FR9: Developers can configure module parameters declaratively (no code generation)

### OpenAPI Ingestion & Processing

- FR10: System can import OpenAPI specifications (JSON/YAML format) from URLs or files
- FR11: System can parse OpenAPI specifications to extract endpoints, schemas, and types
- FR12: System can extract authentication requirements from OpenAPI specifications (API key, OAuth2 basic)
- FR13: System can handle REST API specifications (primary protocol for MVP)
- FR14: System can extract data schemas and field definitions from OpenAPI specifications
- FR15: System can identify required and optional fields from OpenAPI schemas

### Automatic Connector Generation

- FR17: System can generate a declarative connector pipeline from two OpenAPI specifications
- FR18: System can generate initial Input module (HTTP polling) from source OpenAPI specification
- FR19: System can generate initial Output module (HTTP request) from target OpenAPI specification
- FR20: System can generate initial Filter module (mapping) with field-to-field mappings between source and target schemas
- FR21: System can generate connector declarations in explicit, readable format (YAML/JSON)
- FR22: System can generate connector declarations that are diffable and versionable
- FR23: System can generate connector declarations with explicit module configurations
- FR24: System can generate connector declarations that are manually editable by developers
- FR25: System can generate connector declarations with authentication configurations (API key, OAuth2 basic)
- FR26: System can generate connector declarations with pagination handling
- FR27: System can generate connector declarations with error handling patterns
- FR28: System can generate connector declarations with retry logic configurations
- FR29: System can generate connector declarations with CRON scheduling for polling inputs

### AI-Assisted Mapping

- FR30: System can suggest probable field mappings between source and target schemas (e.g., customer_id → client_id)
- FR31: System can display confidence levels for suggested mappings
- FR32: Developers can accept or reject AI-suggested mappings
- FR33: System can suggest mappings for common data types (dates, enums, amounts)
- FR34: System can suggest mappings based on field name similarities
- FR35: Developers can manually override any AI-suggested mapping
- FR36: System uses AI only for generation assistance, not for execution (deterministic runtime)

### Connector Execution

- FR37: Developers can execute a connector pipeline using CLI command (e.g., `connector run my-connector.yaml`)
- FR38: System can execute connectors in dry-run mode (no side effects on target system)
- FR39: System can execute connectors in production mode (actual data transfer)
- FR40: Runtime can execute Input modules to retrieve data from source systems
- FR41: Runtime can execute Filter modules to apply mappings, transformations, and conditions
- FR42: Runtime can execute Output modules to send data to target systems
- FR43: System can execute connectors deterministically (predictable, repeatable results)
- FR44: System can handle authentication with source system (API key, OAuth2 basic)
- FR45: System can handle authentication with target system (API key, OAuth2 basic)
- FR46: System can execute connectors with error handling and retry logic
- FR47: System can generate execution logs with clear, explicit messages
- FR48: System can detect and report mapping errors during execution
- FR49: System can validate connector configuration before execution
- FR50: Developers can view execution history for a connector
- FR51: Developers can view execution logs for a specific execution
- FR52: Runtime can execute Input modules with CRON scheduling (polling)
- FR53: Runtime can execute Input modules with webhook reception (real-time)

### Documentation Generation

- FR54: System can generate human-readable documentation of connector pipeline (Input/Filter/Output modules)
- FR55: Generated documentation shows source fields mapped to target fields
- FR56: Generated documentation shows transformations applied to data
- FR57: Generated documentation shows module configurations and flow
- FR58: Generated documentation is suitable for client validation
- FR59: Generated documentation is suitable for audit purposes
- FR60: Generated documentation is suitable for knowledge transfer
- FR61: Developers can export generated documentation in standard formats

### Input Modules

**MVP Modules:**
- FR62: System can configure HTTP Request Input module with polling and CRON scheduling
- FR63: System can configure Webhook Input module for real-time data reception
- FR64: Runtime can execute HTTP Request Input module to fetch data from REST APIs
- FR65: Runtime can execute Webhook Input module to receive HTTP POST requests

**Post-MVP Modules (hors scope MVP):**
- FR66: System can configure SQL Query Input module with polling and CRON scheduling (post-MVP)
- FR67: System can configure Pub/Sub / Kafka Input module (post-MVP)
- FR68: Runtime can execute SQL Query Input module to fetch data from databases (post-MVP)
- FR69: Runtime can execute Pub/Sub / Kafka Input module to consume messages (post-MVP)

### Filter Modules

**MVP Modules:**
- FR70: System can configure Mapping Filter module with declarative field-to-field mappings (OpenAPI-driven)
- FR71: System can configure Condition Filter module with simple if/else logic
- FR72: Runtime can execute Mapping Filter module to transform data according to mappings
- FR73: Runtime can execute Condition Filter module to route or filter data based on conditions

**Post-MVP Modules (hors scope MVP):**
- FR74: System can configure Advanced Transformation Filter module (post-MVP)
- FR75: System can configure Cloning / Fan-out Filter module (post-MVP)
- FR76: System can configure External Query Filter module for dependent API calls (post-MVP)
- FR77: System can configure Scripting Filter module for custom logic (post-MVP)
- FR78: Runtime can execute Advanced Transformation Filter module (post-MVP)
- FR79: Runtime can execute Cloning / Fan-out Filter module (post-MVP)
- FR80: Runtime can execute External Query Filter module (post-MVP)
- FR81: Runtime can execute Scripting Filter module (post-MVP)

### Output Modules

**MVP Modules:**
- FR82: System can configure HTTP Request Output module for sending data to REST APIs
- FR83: Runtime can execute HTTP Request Output module to send data via HTTP requests

**Post-MVP Modules (hors scope MVP):**
- FR84: System can configure Webhook Output module (post-MVP)
- FR85: System can configure SQL Output module for writing to databases (post-MVP)
- FR86: System can configure Pub/Sub / Kafka Output module (post-MVP)
- FR87: Runtime can execute Webhook Output module (post-MVP)
- FR88: Runtime can execute SQL Output module (post-MVP)
- FR89: Runtime can execute Pub/Sub / Kafka Output module (post-MVP)

### User & Organization Management

- FR90: Users can create an account
- FR91: Users can create an organization
- FR92: Users can belong to multiple organizations
- FR93: Users can switch between organizations
- FR94: Organization Owners can manage organization members
- FR95: Organization Owners can assign roles (Owner, Member) to organization members
- FR96: Organization Owners can remove members from organization
- FR97: Organization Owners can manage organization settings
- FR98: Organization Owners can manage organization subscription
- FR99: System can isolate data by organization (strict logical isolation)
- FR100: System can enforce organization-based data access (users can only access their organization's data)
- FR101: Organization Members can create and manage connectors in their organization
- FR102: Organization Members can view organization connectors and executions
- FR103: Organization Members cannot manage organization settings or members

### Subscription & Billing

- FR104: System can provide Free tier with usage limits
- FR105: System can provide Paid tier with unlimited usage
- FR106: System can enforce Free tier limits (limited connectors per month, individual usage only)
- FR107: System can allow team usage on Paid tier
- FR108: Organization Owners can upgrade from Free to Paid tier
- FR109: Organization Owners can downgrade from Paid to Free tier
- FR110: System can track usage against subscription tier limits
- FR111: System can block usage when Free tier limits are exceeded

### Integration & Workflow

- FR112: System provides CLI tool for connector operations (create, edit, execute)
- FR113: CLI tool can be installed on developer's local machine
- FR114: CLI tool works on multiple platforms (Windows, Mac, Linux)
- FR115: Developers can integrate connector execution into CI/CD pipelines
- FR116: System provides CLI commands for connector management (list, view, execute)
- FR117: Developers can add connector declarations (fichiers YAML/JSON) to their existing production inventory (compatible avec systèmes de versioning standards)

### Template & Reusability

- FR120: Developers can save a connector as a template
- FR121: Developers can create a new connector from an existing template
- FR122: Developers can modify a template-based connector
- FR123: System can organize connectors by project or category
- FR124: Developers can share connector templates within their organization (MVP scope: within organization only)
- FR125: Developers can reuse individual modules (Input/Filter/Output) across multiple connectors

## Non-Functional Requirements

### Performance

**Génération de connecteurs :**
- Le temps moyen pour générer un premier connecteur doit être <4 heures
- La génération automatique d'un connecteur déclaratif à partir de deux OpenAPI doit se compléter en <30 minutes pour des spécifications typiques (50-200 endpoints)
- L'affichage des suggestions IA assistive pour le mapping doit se compléter en <10 secondes

**Exécution runtime :**
- Le runtime doit exécuter des connecteurs avec une latence acceptable pour des transferts de données typiques
- Les logs d'exécution doivent être générés en temps réel sans impact significatif sur les performances

**API et interface :**
- Les opérations CRUD sur les connecteurs (list, view, edit) doivent se compléter en <2 secondes
- L'authentification utilisateur doit se compléter en <1 seconde

### Security

**Isolation des données :**
- Le système doit isoler strictement les données par organisation (isolation logique multi-tenant)
- Aucune fuite de données entre organisations n'est autorisée (validation systématique de l'appartenance organisation)
- Les données doivent être isolées au niveau base de données avec organisation_id sur toutes les tables

**Authentification et autorisation :**
- Toutes les communications doivent utiliser HTTPS (chiffrement en transit)
- Les mots de passe doivent être stockés de manière sécurisée (hashing, pas de stockage en clair)
- Les sessions utilisateur doivent être sécurisées avec tokens sécurisés
- L'authentification multi-facteur doit être supportée (MVP: optionnel, post-MVP: recommandé)

**Conformité :**
- Le système doit être conforme GDPR de base (données personnelles, droit à l'effacement)
- Les utilisateurs doivent pouvoir supprimer leur compte et toutes leurs données associées
- Les logs d'audit doivent tracer les accès aux données sensibles (MVP: basique, post-MVP: complet)

**Intégrations externes :**
- Les credentials API (API keys, OAuth tokens) doivent être stockés de manière sécurisée (chiffrement au repos)
- Les connexions vers systèmes externes doivent utiliser HTTPS/TLS

### Scalability

**Capacité utilisateurs :**
- Le système doit supporter 50-100 utilisateurs actifs simultanés au MVP
- Le système doit être conçu pour supporter 500-1000 utilisateurs actifs à 12 mois
- Le système doit être conçu pour supporter 2000-5000 utilisateurs actifs à 24 mois
- L'architecture doit permettre une montée en charge progressive sans refonte majeure

**Capacité connecteurs :**
- Le système doit supporter 100-200 connecteurs créés au MVP
- Le système doit supporter 5000+ connecteurs à 12 mois
- Le système doit supporter 20000+ connecteurs à 24 mois

**Performance avec croissance :**
- Les performances ne doivent pas dégrader de plus de 20% avec 10x plus d'utilisateurs (objectif <10%)
- L'isolation multi-tenant doit rester efficace avec croissance du nombre d'organisations

### Reliability

**Déterminisme runtime :**
- Le runtime doit être 100% déterministe (exécutions prévisibles, pas de comportements aléatoires)
- Le même connecteur avec les mêmes données d'entrée doit produire les mêmes résultats à chaque exécution

**Qualité génération :**
- >95% des connecteurs générés doivent être fonctionnels dès la première itération (sans corrections majeures)
- Les connecteurs générés doivent être valides syntaxiquement et sémantiquement

**Disponibilité :**
- Le système doit avoir une disponibilité ≥99% (MVP: objectif, post-MVP: SLA)
- Les temps d'arrêt planifiés doivent être minimisés et communiqués à l'avance
- Le système doit récupérer automatiquement des erreurs transitoires

**Gestion d'erreurs :**
- Le système doit gérer les erreurs de manière robuste et explicite
- Les erreurs doivent être loggées avec suffisamment de contexte pour debugging
- Les erreurs d'exécution de connecteur ne doivent pas causer de perte de données

### Integration

**OpenAPI :**
- Le système doit supporter les spécifications OpenAPI 3.0 (JSON/YAML)
- Le système doit gérer les spécifications OpenAPI avec jusqu'à 500 endpoints par API (MVP: support typique 50-200)
- Le système doit être extensible pour supporter versions futures OpenAPI

**Versioning :**
- Les déclarations de connecteur doivent être en format texte (YAML/JSON), diffable et auditable
- Les déclarations doivent être compatibles avec systèmes de versioning standards (développeurs peuvent ajouter les fichiers dans leur inventaire de production déjà versionné)

**CLI :**
- Le CLI doit fonctionner sur Windows, Mac, et Linux
- Le CLI doit s'installer en <15 minutes (documentation et runtime portable)
- Le CLI doit être compatible avec scripts d'automation standards (bash, PowerShell)

**CI/CD :**
- Les connecteurs doivent pouvoir être exécutés dans des pipelines CI/CD standards
- Le runtime CLI doit être compatible avec exécution dans conteneurs Docker
- Les intégrations CI/CD ne doivent pas nécessiter de modifications majeures des workflows existants

**Compatibilité :**
- Le format déclaratif doit être backward compatible entre versions du runtime (format stable)
- Les déclarations doivent rester lisibles et éditables avec éditeurs texte standards (YAML/JSON)

### Maintainability

**Format déclaratif :**
- Les connecteurs déclaratifs doivent rester lisibles et maintenables dans le temps
- Le format déclaratif doit être stable et backward compatible
- Les déclarations doivent être versionnables (format texte, diffable, compatible avec systèmes de versioning standards)

**Runtime :**
- Le runtime doit être maintenable comme composant unique (pas de dépendances frameworks externes dans déclarations générées)
- Le runtime doit pouvoir évoluer indépendamment des déclarations (format déclaratif stable)
- Les mises à jour runtime ne doivent pas casser les déclarations existantes (backward compatibility)

## Project Scoping & Phased Development

### MVP Strategy & Philosophy

**MVP Approach:** Problem-Solving MVP - Résoudre le problème core avec features minimales

**Philosophie MVP :**
- Focus sur résoudre le problème central : permettre à un développeur de créer un connecteur fonctionnel en quelques heures au lieu de plusieurs jours
- Minimum nécessaire pour prouver la valeur : génération automatique + runtime fonctionnel
- Validation de l'hypothèse core : les développeurs veulent-ils automatisation complète avec contrôle total ?
- Apprentissage validé : réduction temps mesurable et adoption naturelle

**Resource Requirements MVP :**
- Équipe : 2-4 développeurs (fullstack/backend)
- Compétences : Backend (Go/Rust), OpenAPI parsing, IA/ML basique, CLI development
- Timeline : 3-6 mois pour MVP (selon équipe)

### Development Priority & Epic Order

**Ordre de développement des epics (tous prioritaires, mais séquentiels) :**

Avec l'évolution vers un modèle de pipeline modulaire (Input → Filter → Output), l'ordre de développement est critique pour garantir la cohérence du format de configuration et l'efficacité du développement.

**1. Définition du format du fichier de configuration (Priorité 1 - Epic 1)**
- **Objectif** : Définir le format exact et complet du fichier de configuration déclarative (YAML/JSON) pour les pipelines modulaires
- **Livrables** : 
  - Spécification complète du format déclaratif (schéma, validation, exemples)
  - Documentation du format pour tous les modules (Input/Filter/Output)
  - Exemples de configurations complètes et valides
- **Rationale** : Le format de configuration est la fondation de tout le système. Il doit être défini en premier pour que le CLI (ingesteur de conf) et le Front (générateur de conf) puissent s'y aligner.

**2. CLI - Ingesteur de configuration (Priorité 2 - Epic 2)**
- **Objectif** : Développer le CLI qui ingère, valide et exécute les fichiers de configuration
- **Livrables** :
  - CLI qui lit et valide les fichiers de configuration selon le format défini
  - Runtime qui exécute les pipelines déclaratifs (modules Input/Filter/Output)
  - Validation complète des configurations
  - Gestion d'erreurs et logs explicites
- **Rationale** : Le CLI est l'ingesteur de configuration qui va définir le format exact en pratique. En développant le CLI en second, on valide le format défini et on l'ajuste si nécessaire avant de développer le Front. Le CLI permet aussi de tester et valider le runtime indépendamment de l'interface utilisateur.

**3. Front - Générateur de configuration (Priorité 3 - Epic 3)**
- **Objectif** : Développer l'interface utilisateur (web) qui génère les fichiers de configuration
- **Livrables** :
  - Interface web pour création/modification de connecteurs
  - Génération automatique de configurations depuis OpenAPI
  - IA assistive pour suggestions de mapping
  - Export des configurations générées (fichiers YAML/JSON)
- **Rationale** : Le Front génère les fichiers de configuration selon le format défini et validé par le CLI. En développant le Front en dernier, on s'assure que le format est stable et que le CLI peut valider toutes les configurations générées par le Front.

**Approche de développement :**
- **Séquentiel strict** : Epic 1 → Epic 2 → Epic 3 (pour garantir cohérence du format)
- **Validation intermédiaire** : Le CLI (Epic 2) doit être fonctionnel avec des configurations générées manuellement avant de passer au Front (Epic 3)
- **Validation continue** : Le CLI (Epic 2) doit pouvoir valider toutes les configurations générées par le Front (Epic 3)

**Critères de passage Epic 2 → Epic 3 :**
- Le CLI est fonctionnel et peut exécuter des pipelines complets avec configurations manuelles
- Le format de configuration est validé et stable
- Le runtime exécute correctement tous les modules MVP (Input HTTP/Webhook, Filter Mapping/Conditions, Output HTTP)
- Les configurations manuelles sont testées et validées

**Impact sur le MVP :**
- Le Front est inclus dans le MVP (génération automatique depuis OpenAPI + mapping IA assistive)
- Le CLI doit être fonctionnel avec configurations manuelles avant le développement du Front
- Cette approche garantit que le format est stable et validé avant que le Front ne génère des configurations

### MVP Feature Set (Phase 1)

**Core User Journeys Supported:**
- Marc (Consultant ERP) : Création connecteur pour migration critique
- Alex (Développeur SaaS B2B) : Standardisation intégrations clients
- Sophie (Tech Lead) : Évaluation outil pour équipe

**Must-Have Capabilities (indispensables) :**

1. **Ingestion OpenAPI (source + cible)**
   - Import spécifications OpenAPI REST (JSON/YAML)
   - Extraction endpoints, schémas, types
   - Support REST + JSON + Auth standard (API key/OAuth2 basique)

2. **Génération pipeline déclaratif modulaire**
   - Génération automatique pipeline connecteur avec modules Input/Filter/Output
   - Format explicite, lisible, diffable (YAML/JSON)
   - Pipeline modifiable manuellement

3. **IA assistive mapping (suggestion uniquement)**
   - Suggestions correspondances probables (customer_id → client_id, dates, enums, montants)
   - Afficher niveau de confiance
   - Aucune décision automatique non validée

4. **Runtime portable minimal (CLI)**
   - Runtime exécutable CLI : `connector run my-connector.yaml`
   - Capable d'exécuter modules Input (HTTP polling, webhook), Filter (mapping, conditions), Output (HTTP)
   - Logs clairs + erreurs explicites

5. **Mode test / dry-run**
   - Exécution sans effet côté cible
   - Validation mapping et transformations
   - Détection erreurs avant prod

6. **Documentation automatique mapping**
   - Génération document lisible : champs source → champs cible, transformations appliquées
   - Utilisable pour validation client, audit, passation

**Modules supportés au MVP :**

**Input Modules (MVP) :**
- HTTP Request (polling + CRON) : Récupération périodique de données depuis APIs REST
- Webhook : Réception de données en temps réel via webhooks HTTP

**Filter Modules (MVP) :**
- Mapping déclaratif (OpenAPI-driven) : Mapping champ-à-champ entre schémas source et cible
- Conditions simples (if/else) : Logique conditionnelle basique pour routing et filtrage

**Output Modules (MVP) :**
- HTTP Request : Envoi de données vers APIs REST via requêtes HTTP

**Infrastructure MVP :**
- Multi-tenant simple (isolation logique par organisation)
- Permissions simplifiées (Owner/Member)
- Free + Paid tiers
- Intégration CLI + Front (tous deux inclus dans le MVP)

**Ordre de développement MVP :**
1. **Format de configuration** : Définition complète du format déclaratif (YAML/JSON) pour pipelines modulaires
2. **CLI** : Ingesteur de configuration et runtime d'exécution (prioritaire - doit être fonctionnel avec configurations manuelles avant de passer au Front)
3. **Front** : Interface web de génération de configurations depuis OpenAPI avec mapping IA assistive (développé après validation du CLI avec configurations manuelles)

**Out of Scope MVP (volontairement) :**

**Modules hors scope MVP :**
- **SQL Input/Output** : Pas de support bases de données relationnelles au MVP
- **Pub/Sub / Kafka** : Pas de support systèmes de messagerie asynchrone au MVP
- **Scripting avancé** : Pas de support scripts personnalisés au MVP
- **Transformations avancées** : Pas de transformations complexes (formatting, calculs, agrégations) au MVP
- **Cloning / Fan-out** : Pas de duplication de données vers plusieurs sorties au MVP
- **Requêtes externes dépendantes** : Pas d'appels API conditionnels basés sur valeurs d'entrée au MVP

**Autres limitations MVP :**
- UI graphique avancée (CLI + fichiers suffisent)
- Support multi-protocoles (REST + JSON uniquement)
- Orchestration complexe (1 source → 1 cible uniquement, pipeline linéaire simple)
- Runtime managé obligatoire (CLI locale/CI uniquement)
- Auto-adaptation production (modifications explicites validées)
- Plugins IDE (post-MVP)
- Compliance Enterprise (GDPR basique uniquement)

**Objectif MVP :** Prouver que le runtime interprétant un pipeline déclaratif modulaire est viable, que la génération OpenAPI → pipeline fonctionne, et que le développeur garde un contrôle total et une compréhension complète.

**Critères de succès MVP :**
- 50-100 utilisateurs beta actifs
- ≥70% déclarent gain temps significatif
- NPS ≥35
- Cas réels migration/intégration livrés
- "J'ai livré un connecteur fiable en quelques heures, pas en plusieurs jours."

### Post-MVP Features

**Phase 2 (Post-MVP - Growth) :**

**Nouveaux modules Input/Output :**
- **SQL Input/Output** : Support bases de données relationnelles (SQL queries avec polling + CRON, écriture SQL)
- **Pub/Sub / Kafka** : Intégration systèmes de messagerie asynchrone (consommation et publication)

**Nouveaux modules Filter :**
- **Transformations avancées** : Transformations complexes (formatting, calculs, agrégations)
- **Cloning / Fan-out** : Duplication de données vers plusieurs sorties
- **Requêtes externes dépendantes** : Appels API conditionnels basés sur valeurs d'entrée
- **Scripting avancé** : Support scripts personnalisés pour logique métier complexe

**Autres features :**
- **Plugins VSCode/CI-CD** : Intégration native outils développeur
- **Mapping IA avancé** : Transformations complexes, détection patterns avancée
- **Monitoring et observabilité** : Insights performance intégrations, alertes proactives
- **Support formats additionnels** : XML, transformations plus complexes
- **Multi-sources basique** : 2-3 sources vers 1 cible (pipeline avec plusieurs Input modules)
- **Bibliothèque templates** : Connecteurs pré-configurés APIs populaires (Salesforce, SAP, etc.)
- **Collaboration** : Partage connecteurs, templates communautaires
- **RBAC avancé** : Permissions fines par ressource
- **Compliance Enterprise** : SOC2, ISO27001 si nécessaire

**Phase 3 (Expansion) :**
- **UI avancée mapping** : Éditeur visuel création/modification connecteurs
- **Librairies runtime par langage** : Runtime Go, Rust, Node.js, Python, etc.
- **Support cas complexes** : Multi-sources avancé, règles conditionnelles, transformations avancées
- **Exécution managée optionnelle** : Plateforme SaaS optionnelle exécution cloud, monitoring
- **Marketplace connecteurs** : Bibliothèque réutilisable connecteurs pré-configurés
- **Intégration verticale** : Solutions sectorielles (healthtech, fintech, e-commerce)
- **Isolation physique multi-tenant** : Sharding par organisation si nécessaire

### Risk Mitigation Strategy

**Technical Risks :**

**Risque 1 : Runtime portable performance/déterminisme**
- **Risque** : Runtime pas suffisamment performant ou déterministe
- **Mitigation** : Tests rigoureux différentes plateformes, optimisation continue, choix langage approprié (Go/Rust)
- **Contingency** : Adapter runtime ou proposer variantes par plateforme

**Risque 2 : IA assistive insuffisante**
- **Risque** : IA ne propose pas suggestions utiles
- **Mitigation** : Itération modèles IA, amélioration détection patterns, feedback utilisateur continu
- **Contingency** : MVP fonctionne même sans IA assistive efficace (génération automatique base reste utile)

**Risque 3 : Format déclaratif complexité perçue**
- **Risque** : Développeurs perçoivent approche déclarative comme complexe
- **Mitigation** : Documentation claire, exemples concrets, onboarding rapide <15 min
- **Contingency** : Simplifier format déclaratif ou améliorer génération automatique

**Market Risks :**

**Risque 4 : Non-adoption développeurs**
- **Risque** : Développeurs préfèrent solutions existantes
- **Mitigation** : Démonstration claire valeur (gain temps mesurable), intégration workflow développeur, freemium généreux
- **Contingency** : Pivoter positionnement (embedded iPaaS, outils internes)

**Risque 5 : Compétition iPaaS établis**
- **Risque** : iPaaS établis ajoutent fonctionnalités similaires
- **Mitigation** : Vitesse exécution, focus développeur, meilleure UX dev, avantage first-mover
- **Contingency** : Pivoter embedded iPaaS ou acquisition par acteur établi

**Resource Risks :**

**Risque 6 : Ressources limitées**
- **Risque** : Équipe plus petite que prévu
- **Mitigation** : MVP scope minimal strict, prioritisation features must-have uniquement
- **Contingency** : Simplifier MVP encore plus (sans IA assistive initialement, runtime minimal uniquement)

**Risque 7 : Timeline dépassée**
- **Risque** : Développement prend plus de temps que prévu
- **Mitigation** : MVP scope strict, itérations rapides, validation précoce hypothèses
- **Contingency** : Sortir MVP avec features réduites, itérer rapidement basé feedback

## Product Scope

### MVP - Minimum Viable Product

**Objectif MVP :** Prouver la valeur avec le minimum nécessaire pour qu'un développeur puisse créer un connecteur fonctionnel en quelques heures au lieu de plusieurs jours.

**Ordre de développement MVP :**
1. **Format de configuration** : Définition complète du format déclaratif (YAML/JSON) pour pipelines modulaires
2. **CLI** : Ingesteur de configuration et runtime d'exécution (prioritaire - doit être fonctionnel avec configurations manuelles avant de passer au Front)
3. **Front** : Interface web de génération de configurations depuis OpenAPI avec mapping IA assistive (développé après validation du CLI avec configurations manuelles)

**Note :** Le CLI doit être fonctionnel et validé avec des configurations générées manuellement avant que l'équipe ne passe au développement du Front. Le Front est inclus dans le MVP et génère automatiquement les configurations depuis OpenAPI avec mapping IA assistive.

**Features MVP (indispensables) :**

1. **Ingestion de deux OpenAPI (source + cible)**
   - Import de spécifications OpenAPI REST (JSON/YAML)
   - Extraction des endpoints, schémas et types
   - Support cas simple mais réel : REST, JSON, Auth standard (API key / OAuth2 basique)

2. **Génération d'un pipeline déclaratif modulaire initial**
   - Génération automatique d'un pipeline connecteur avec modules Input/Filter/Output
   - Structure explicite, lisible, diffable
   - Pipeline modifiable manuellement dès le départ

3. **IA assistive pour le mapping (suggestion uniquement)**
   - Proposer des correspondances probables (customer_id → client_id, dates, enums, montants)
   - Afficher une confidence
   - Aucune décision automatique non validée

4. **Runtime portable minimal (CLI)**
   - Runtime exécutable via CLI : `connector run my-connector.yaml`
   - Capable d'exécuter modules Input (HTTP polling, webhook), Filter (mapping, conditions), Output (HTTP)
   - Logs clairs + erreurs explicites

5. **Mode test / dry-run**
   - Exécution sans effet côté cible
   - Validation du mapping et des transformations
   - Détection d'erreurs avant prod

6. **Documentation automatique du mapping**
   - Génération d'un document lisible : champs source → champs cible, transformations appliquées
   - Utilisable pour validation client, audit, passation

**Modules supportés au MVP :**

**Input Modules :**
- HTTP Request (polling + CRON) : Récupération périodique de données depuis APIs REST
- Webhook : Réception de données en temps réel via webhooks HTTP

**Filter Modules :**
- Mapping déclaratif (OpenAPI-driven) : Mapping champ-à-champ entre schémas source et cible
- Conditions simples (if/else) : Logique conditionnelle basique pour routing et filtrage

**Output Modules :**
- HTTP Request : Envoi de données vers APIs REST via requêtes HTTP

**Out of Scope MVP (volontairement) :**

**Modules explicitement hors scope MVP :**
- **SQL Input/Output** : Pas de support bases de données relationnelles
- **Pub/Sub / Kafka** : Pas de support systèmes de messagerie asynchrone
- **Scripting libre** : Pas de support scripts personnalisés
- **Multi-input / multi-output complexes** : Pipeline linéaire simple uniquement (1 Input → Filter → 1 Output)

**Autres limitations MVP :**
- UI graphique avancée (CLI + fichiers suffisent)
- Support multi-protocoles (REST + JSON uniquement)
- Orchestration complexe (pipeline linéaire simple uniquement)
- Runtime managé obligatoire (CLI locale / CI uniquement)
- Auto-adaptation en production (toutes modifications explicites et validées)

**Critères de succès MVP :**
- 50-100 utilisateurs beta actifs
- ≥70% déclarent un gain de temps significatif
- NPS ≥35
- Cas réels de migration/intégration livrés
- "J'ai livré un connecteur fiable en quelques heures, pas en plusieurs jours."

### Growth Features (Post-MVP)

**Features de croissance (après validation MVP) :**

**Nouveaux modules Input/Output :**
- **SQL Input/Output** : Support bases de données relationnelles (SQL queries avec polling + CRON, écriture SQL)
- **Pub/Sub / Kafka** : Intégration systèmes de messagerie asynchrone (consommation et publication)

**Nouveaux modules Filter :**
- **Transformations avancées** : Transformations complexes (formatting, calculs, agrégations)
- **Cloning / Fan-out** : Duplication de données vers plusieurs sorties
- **Requêtes externes dépendantes** : Appels API conditionnels basés sur valeurs d'entrée
- **Scripting avancé** : Support scripts personnalisés pour logique métier complexe

**Autres features :**
- **Plugins VSCode/CI-CD** : Intégration native dans les outils développeur
- **Mapping IA avancé** : Transformations plus complexes, détection patterns avancée
- **Monitoring et observabilité** : Insights sur performance intégrations, alertes proactives
- **Support formats additionnels** : XML, transformations plus complexes
- **Multi-sources basique** : 2-3 sources vers 1 cible (pipeline avec plusieurs Input modules)
- **Bibliothèque templates** : Connecteurs pré-configurés pour APIs populaires (Salesforce, SAP, etc.)
- **Collaboration** : Partage de connecteurs, templates communautaires

### Vision (Future)

**Vision 2-3 ans (si MVP fonctionne) :**

**Modules avancés :**
- **Support complet SQL** : Tous types de bases de données (PostgreSQL, MySQL, SQL Server, etc.)
- **Support complet messaging** : Kafka, RabbitMQ, AWS SQS, Google Pub/Sub, etc.
- **Modules Filter avancés** : Machine learning pour transformations intelligentes, agrégations complexes
- **Modules Input/Output personnalisés** : Framework pour créer modules custom par développeurs

**Architecture et plateforme :**
- **UI avancée de mapping** : Éditeur visuel pour faciliter création et modification de pipelines modulaires
- **Librairies runtime par langage** : Runtime disponible en Go, Rust, Node.js, Python, etc.
- **Support de cas complexes** : Multi-sources avancé, pipelines parallèles, règles conditionnelles complexes
- **Exécution managée optionnelle** : Plateforme SaaS optionnelle pour exécution cloud, monitoring, etc.
- **Marketplace de connecteurs** : Bibliothèque réutilisable de connecteurs pré-configurés pour APIs populaires
- **Marketplace de modules** : Bibliothèque de modules Input/Filter/Output réutilisables et partagés

**Intégration et écosystème :**
- **Intégration verticale** : Solutions sectorielles (healthtech, fintech, e-commerce)
- **Orchestration avancée** : Gestion de pipelines complexes avec dépendances, scheduling avancé
- **Observabilité native** : Monitoring, tracing, métriques intégrés dans le runtime

**Note :** Rien de tout cela n'est requis pour prouver la valeur dans le MVP. Le MVP se concentre sur prouver la viabilité du modèle de pipeline modulaire déclaratif avec modules Input/Filter/Output de base.
