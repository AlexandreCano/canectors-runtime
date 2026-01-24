# Sprint Change Proposal - CLI Runtime Refinements

**Date:** 2026-01-24  
**Auteur:** Cano (via Scrum Master Bob)  
**Statut:** En attente d'approbation

---

## Section 1: Issue Summary

### Problème Déclencheur

Suite à la finalisation de l'implémentation des features MVP du CLI (fin de la story 4.4), une discussion avec un collègue a révélé que le CLI est le cœur du projet et nécessite des corrections/fonctionnalités supplémentaires pour être parfaitement utilisable avant de passer à la partie site web.

### Contexte de Découverte

**Quand:** Après la complétion de la story 4.4 (Implement Error Handling and Retry Logic)  
**Comment:** Discussion avec un collègue sur l'utilité du produit  
**Impact immédiat:** Changement de scope - nécessité d'ajouter 3 nouveaux epics avant de continuer avec les stories 4.5 et 4.6

### Problème Précis

Le CLI runtime MVP est fonctionnel mais manque de :
- **Corrections mineures** (5 items) : Simplifications et améliorations de comportement
- **Corrections majeures** (3 items) : Améliorations architecturales pour extensibilité
- **Nouvelles fonctionnalités** (6 items) : Capacités avancées pour production

### Évidence

**Scénarios identifiés :**
- `schemaVersion` exposé sans mécanisme de migration réel
- Input module reste ouvert pendant l'exécution des filters
- Exécutions concurrentes non coordonnées
- Erreurs "unknown" non retryables par défaut
- Schedule défini au niveau connecteur au lieu de l'input
- Extensibilité des modules nécessite modification du core
- Boundaries des modules pas assez claires
- Retry des outputs non configurable selon la réponse
- Pas de scheduler distribué
- Pas de configuration de connection pooling
- Pas de persistence de timestamp pour polling
- Pas de support scripting
- Pas d'enrichissement dynamique dans les filters

---

## Section 2: Impact Analysis

### Epic Impact

**Epics affectés :**
- **Epic 1** (done) : Pas d'impact direct
- **Epic 2** (done) : Pas d'impact direct
- **Epic 3** (done) : Pas d'impact direct
- **Epic 4** (in-progress) : Stories 4-5 et 4-6 reportées après Epic 14

**Nouveaux epics requis :**
- **Epic 12** : CLI Runtime Refinements (Minor Corrections) - 5 stories
- **Epic 13** : Module Architecture Improvements (Major Corrections) - 3 stories
- **Epic 14** : Advanced Runtime Capabilities (New Features) - 6 stories

### Story Impact

**Stories actuelles :**
- Stories 4-1 à 4-4 : Complétées, pas d'impact
- Stories 4-5 et 4-6 : Reportées (backlog, non prioritaires pour l'instant)

**Nouvelles stories à créer :**
- **Epic 12** : 5 stories (minor corrections)
- **Epic 13** : 3 stories (major corrections)
- **Epic 14** : 6 stories (new features)

**Total : 14 nouvelles stories**

### Artifact Conflicts

**PRD :**
- ✅ Pas de conflit - Les changements renforcent les objectifs MVP (runtime portable, déterministe, extensible)
- Extension du scope CLI mais alignée avec la vision produit
- MVP reste atteignable avec scope CLI élargi

**Architecture :**
- ⚠️ Sections à mettre à jour :
  - Module Execution patterns (extensibilité, boundaries)
  - Runtime Architecture (scheduler distribué, connection pooling)
  - CLI Architecture (module registration, plugin-like)

**UX Design :**
- ✅ Pas d'impact direct - Ces changements sont CLI-only

**Autres artifacts :**
- ⚠️ Impact sur :
  - Tests (nouveaux modules, patterns d'extensibilité)
  - Documentation (extensibility guide, module development)
  - CI/CD (distributed scheduler, configuration)

### Technical Impact

**Code impact :**
- Modifications dans le runtime CLI Go
- Refactoring de l'architecture des modules
- Ajout de nouvelles capacités (scheduler distribué, scripting, etc.)

**Dépendances :**
- Aucune nouvelle dépendance externe majeure identifiée
- Potentielle intégration avec systèmes de scheduling distribués (Quartz, Kubernetes CronJob)

---

## Section 3: Recommended Approach

### Option Évaluée : Direct Adjustment

**Approche sélectionnée :** Option 1 - Direct Adjustment

**Justification :**
- ✅ Les changements sont des extensions naturelles du CLI runtime
- ✅ Pas de conflit avec le MVP défini
- ✅ Effort raisonnable (Medium) - 3 nouveaux epics, 14 stories total
- ✅ Risque faible (Low) - Extensions logiques, pas de breaking changes majeurs
- ✅ Ordre logique : 12 → 13 → 14 → stories 4-5 et 4-6

**Effort estimé :** Medium  
**Risque :** Low  
**Timeline impact :** Extension du scope CLI avant passage au frontend

### Alternatives Considérées

**Option 2 : Potential Rollback**
- ❌ Non viable - Pas besoin de rollback, les epics 1-4 sont solides
- Les changements sont des améliorations, pas des corrections d'erreurs

**Option 3 : PRD MVP Review**
- ❌ Non nécessaire - MVP reste atteignable
- Scope CLI étendu mais aligné avec la vision produit

---

## Section 4: Detailed Change Proposals

### Epic 12: CLI Runtime Refinements (Minor Corrections)

**Objectif :** Améliorer la qualité et la simplicité du CLI runtime avec des corrections mineures

**Stories proposées :**

1. **Remove schemaVersion field**
   - Supprimer le champ `schemaVersion` de la configuration utilisateur
   - Rationale : N'apporte pas de valeur sans mécanisme de migration
   - Impact : Simplification de la configuration

2. **Close input module after input execution**
   - Fermer l'input immédiatement après la phase d'ingestion
   - Rationale : Libérer les ressources réseau, éviter dépendances implicites
   - Impact : Meilleure gestion des ressources

3. **Queue execution when previous still running**
   - Mettre en file d'attente les nouvelles exécutions si une précédente est en cours
   - Rationale : Comportement déterministe, éviter concurrence implicite
   - Impact : Exécutions sérialisées par pipeline

4. **Treat unknown errors as retryable**
   - Considérer les erreurs "unknown" comme retryable par défaut
   - Rationale : Plus susceptibles d'être transitoires (réseau, I/O, timeouts)
   - Impact : Meilleure robustesse

5. **Move schedule configuration to input level**
   - Déplacer la planification du niveau connecteur au niveau input
   - Rationale : Tous les types d'input ne supportent pas le mode scheduled
   - Impact : Configuration plus cohérente

**FRs couverts :** N/A (refinements)  
**NFRs couverts :** NFR24 (déterminisme), NFR25 (reproductibilité)

---

### Epic 13: Module Architecture Improvements (Major Corrections)

**Objectif :** Améliorer l'architecture des modules pour l'extensibilité et la clarté

**Stories proposées :**

1. **Simplify module extensibility (open-source friendly)**
   - Permettre l'ajout de nouveaux modules sans modifier le core
   - Rationale : Faciliter les contributions open-source
   - Impact : Architecture plugin-like avec registry

2. **Clarify module boundaries without heavy encapsulation**
   - Définir des interfaces minimales et stables pour les modules
   - Rationale : Équilibre entre encapsulation et extensibilité
   - Impact : Documentation claire des responsabilités

3. **Configurable output retry based on response**
   - Permettre la configuration du retry selon la réponse (status code, headers, body)
   - Rationale : Distinguer erreurs transitoires vs définitives
   - Impact : Retry plus intelligent

**FRs couverts :** N/A (améliorations architecturales)  
**NFRs couverts :** NFR50 (maintenabilité), NFR51 (évolutivité)

---

### Epic 14: Advanced Runtime Capabilities (New Features)

**Objectif :** Ajouter des capacités avancées pour un usage en production

**Stories proposées :**

1. **Connection pooling configuration for inputs and outputs**
   - Permettre la configuration du pooling, timeouts, limites
   - Rationale : Optimiser les performances, éviter comportements par défaut non maîtrisés
   - Impact : Contrôle fin des connexions

2. **Scripting support in pipelines**
   - Évaluer l'introduction d'un mécanisme de scripting (JavaScript embarqué, sandboxé)
   - Rationale : Autoriser transformations avancées sans compromettre sécurité
   - Impact : Flexibilité accrue pour cas complexes

3. **Dynamic enrichment inside filters (input inside filter + cache)**
   - Permettre aux filters d'exécuter des requêtes externes avec cache configurable
   - Rationale : Enrichir les records dynamiquement, limiter appels redondants
   - Impact : Patterns d'enrichissement standardisés

4. **Last timestamp persistence for polling inputs**
   - Persister un "last processed timestamp" pour les inputs polling
   - Rationale : Assurer reprises fiables, éviter doublons ou pertes
   - Impact : Fiabilité accrue des reprises

5. **Distributed scheduler support**
   - Introduire un scheduler distribué (Quartz, Kubernetes CronJob)
   - Rationale : Exécutions fiables et scalables en environnements multi-instances
   - Impact : Support de déploiements distribués

6. **Additional advanced feature** (à définir selon priorité)

**FRs couverts :** N/A (nouvelles fonctionnalités)  
**NFRs couverts :** NFR17-23 (scalabilité), NFR24-25 (fiabilité)

---

## Section 5: Implementation Handoff

### Change Scope Classification

**Classification :** Moderate

**Justification :**
- Nécessite réorganisation du backlog (ajout de 3 nouveaux epics)
- Coordination PO/SM nécessaire pour priorisation
- Impact sur l'ordre de développement (12 → 13 → 14 avant 4-5 et 4-6)

### Handoff Recipients

**Product Owner / Scrum Master :**
- Créer les nouveaux epics 12, 13, 14 dans le système de tracking
- Prioriser les nouveaux epics avant les stories 4-5 et 4-6
- Mettre à jour le sprint-status.yaml

**Development Team :**
- Implémenter les 14 nouvelles stories dans l'ordre défini
- Suivre les patterns établis dans les epics précédents

### Success Criteria

**Critères de succès :**
- ✅ 3 nouveaux epics créés et documentés
- ✅ 14 stories créées avec descriptions complètes
- ✅ Sprint-status.yaml mis à jour avec nouveaux epics
- ✅ Ordre de priorité respecté : 12 → 13 → 14
- ✅ Architecture document mise à jour si nécessaire

### Next Steps

1. **Approbation de cette proposition** par Cano ✅
2. **Création des epics** via workflow create-story ou manuellement ✅
3. **Mise à jour du sprint-status.yaml** avec nouveaux epics ✅
4. **Démarrage de l'Epic 12** (première priorité)
5. **Ordre d'exécution :** Epic 12 → Epic 13 → Epic 14 → Stories 4-5 et 4-6

### Timeline Estimate

**Effort total estimé :** Medium (3 epics, 14 stories)  
**Timeline :** À définir selon capacité équipe  
**Blocage :** Aucun identifié

---

## Section 6: Final Review

### Checklist Completion

- [x] Section 1 : Issue Summary - Complétée
- [x] Section 2 : Impact Analysis - Complétée
- [x] Section 3 : Recommended Approach - Complétée
- [x] Section 4 : Detailed Change Proposals - Complétée
- [x] Section 5 : Implementation Handoff - Complétée

### Proposal Accuracy

✅ Toutes les recommandations sont bien supportées par l'analyse  
✅ La proposition est actionnable et spécifique  
✅ Les nouveaux epics sont clairement définis avec leurs stories

### User Approval Required

**Question pour Cano :**  
Approuves-tu cette Sprint Change Proposal pour implémentation ?

**Options :**
- [a] Approve - Procéder avec création des epics 12, 13, 14
- [e] Edit - Modifier certains aspects de la proposition
- [r] Revise - Revoir l'approche ou les priorités

---

**Document généré le :** 2026-01-24  
**Workflow :** correct-course  
**Statut :** ✅ Approuvé par Cano le 2026-01-24

**Actions complétées :**
- ✅ Sprint Change Proposal créé et approuvé
- ✅ Sprint-status.yaml mis à jour avec Epics 12, 13, 14
- ✅ 14 nouvelles stories ajoutées au backlog
- ✅ Ordre de priorité établi : 12 → 13 → 14 → 4-5 et 4-6
