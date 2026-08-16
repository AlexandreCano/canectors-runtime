# Stories — Retours des migrations de connecteurs

Stories issues de la campagne de migration de connecteurs existants vers cannectors.

**Le nom de fichier porte deux nombres, ne pas les confondre :**

- Le préfixe `01` … `17` est la **position dans l'ordre d'implémentation**. Il n'identifie pas la story.
- `25.x` / `26.x` est le **numéro de story**, stable, à citer dans les commits et les PR. Epic 25 = bugs, epic 26 = fonctionnalités. La numérotation prolonge celle en vigueur dans le code (dernière en place : Story 24.12).

L'ordre du répertoire est donc littéralement la feuille de route.

---

## Ordre d'implémentation

### Phase A — Fondations

Rien d'autre ne doit démarrer avant. Ces quatre stories fixent les contrats que les treize suivantes consomment.

| # | Story | Titre | Prérequis |
|---|---|---|---|
| 01 | [26.10](01-26.10-marqueur-erreur-dans-record.md) | Marqueur d'erreur exploitable dans le record (`onError` utile) | — |
| 02 | [25.1](02-25.1-http-url-construction-unifiee.md) | Construction d'URL HTTP unifiée : `queryParams` partout + encodage de l'URL finale | — |
| 03 | [25.4](03-25.4-loop-racine-adressage-unique.md) | `loop` : une seule racine d'adressage | — |
| 04 | [25.5](04-25.5-state-injectable-partout-input.md) | État persistant injectable dans tous les paramètres d'input | 25.1, 25.4 |

**Pourquoi cet ordre :**

- **26.10 en premier** — petit périmètre, et conditionne le *sens* de 26.1, 26.9, 26.7 et 26.3. Router des rejets, décider quels records comptent dans un watermark, répondre à un webhook synchrone : les trois supposent un statut d'erreur lisible dans le record. Les faire avant, c'est les refaire après.
- **25.1 avant 25.5** — injecter l'état dans une URL n'a aucun intérêt si l'URL finale n'est pas encodée. Sinon on livre une fonctionnalité qui produit des requêtes cassées dès que le watermark contient un `+` ou un espace (cas d'un timestamp).
- **25.4 avant 25.5** — 25.4 tranche « quelle est la racine du contexte de rendu ». 25.5 en ajoute un (`state.*`). Ajouter des contextes à un modèle d'adressage encore incohérent revient à multiplier l'incohérence par trois.
- **25.5 est le point de convergence** — `state.*` (25.5), `auth.*` (26.5) et `tenant.*` (26.2) doivent partager **un seul** moteur de contexte. C'est ici qu'il se conçoit ; les deux autres s'y branchent. Les implémenter séparément reproduirait exactement le défaut que 25.4 corrige.

### Phase B — Contrats de filtres

Bugs autonomes, faible risque, aucun couplage avec la phase A. Parallélisables entre eux (sauf 25.3 qui suit 25.2) et parallélisables avec la phase A si tu as plusieurs personnes.

| # | Story | Titre | Prérequis |
|---|---|---|---|
| 05 | [25.2](05-25.2-contrat-resultkey-filtres-appel.md) | Contrat unifié `resultKey` / `mergeStrategy` — **option A actée** | — |
| 06 | [25.3](06-25.3-http-call-datafield-tableau.md) | `http_call` : `dataField` sur un tableau de plus d'un élément | 25.2 |
| 07 | [25.6](07-25.6-output-database-dry-run.md) | Output `database` : pas d'aperçu en `--dry-run` et connexion ouverte quand même | — |

**Pourquoi 25.2 avant 25.3** : la correction de `dataField` a besoin d'une destination configurable pour écrire un tableau multi-éléments. Sans `resultKey` sur `http_call`, on n'a nulle part où le mettre proprement.

### Phase C — Incrémental fiable

C'est la phase qui débloque le plus de migrations réelles.

| # | Story | Titre | Prérequis |
|---|---|---|---|
| 08 | [26.8](08-26.8-mapping-dateformat-timezone.md) | `mapping` / `dateFormat` : option de fuseau horaire | — |
| 09 | [26.9](09-26.9-watermark-max-champ-record.md) | Watermark d'état dérivé des données : `lastTimestamp = max(<champ>)` | 26.10, 25.5, 26.8 |
| 10 | [26.11](10-26.11-pagination-link.md) | Pagination de type `link` (`nextUrlField` / `nextUrlHeader`) | 25.1, 26.9 |

**Pourquoi cet ordre :**

- **26.8 avant 26.9** — les deux parsent des dates et manipulent des fuseaux. 26.8 pose la brique (stdlib, tzdata), 26.9 la consomme. L'inverse conduit à deux parseurs de dates dans la base de code.
- **26.11 après 26.9** — la pagination multi-pages est précisément le cas où le watermark `executionStart` est faux. Tester `link` sans le watermark corrigé, c'est valider une pagination sur une base d'état non fiable (cf. AC14 de 26.11 / AC8 de 26.9).

### Phase D — Authentification

| # | Story | Titre | Prérequis |
|---|---|---|---|
| 11 | [26.5](11-26.5-oauth2-grant-types-et-reponse-token.md) | OAuth2 : jeu complet de grant types + exploitation de la réponse du token | 25.5 |

Placée après la phase C parce qu'elle branche `auth.*` sur le moteur de contexte de 25.5, et avant 26.2 dont elle est un prérequis (cloisonnement du cache de token par tenant).

### Phase E — Topologie du pipeline

Ces deux stories changent la structure du pipeline (`1 output` → N, chaîne de filtres `1→1` → `N→M`). À faire après les fondations, jamais avant.

| # | Story | Titre | Prérequis |
|---|---|---|---|
| 12 | [26.1](12-26.1-multi-output-routage-conditionnel.md) | Outputs multiples avec routage conditionnel | 26.10 |
| 13 | [26.3](13-26.3-cardinalite-split-aggregate.md) | Modules de cardinalité : `split` (1→N) et `aggregate` (N→1) | 26.10, 25.4, 26.9 |

**Pourquoi 26.1 avant 26.3** : 26.3 doit savoir si une agrégation se configure globalement ou par output. Si le multi-output existe déjà, la réponse est évidente (par output, en chaîne pré-output) et 26.3 n'a pas à inventer de mécanisme redondant.

### Phase F — Webhook

Les deux touchent le même handler. À faire dans la même passe pour ne l'ouvrir qu'une fois.

| # | Story | Titre | Prérequis |
|---|---|---|---|
| 14 | [26.6](14-26.6-webhook-authentification-et-routing.md) | Webhook : authentification entrante et routing multi-endpoints | — |
| 15 | [26.7](15-26.7-webhook-reponse-configurable.md) | Webhook : réponse HTTP configurable | 26.10, 26.6 |

**Pourquoi 26.6 avant 26.7** : 26.6 restructure la table de routage et la config d'entrée ; 26.7 greffe la réponse sur des endpoints qui doivent déjà exister au pluriel. L'inverse impose de refaire la logique de réponse par endpoint.

### Phase G — Passage à l'échelle

| # | Story | Titre | Prérequis |
|---|---|---|---|
| 16 | [26.4](16-26.4-factorisation-gabarits.md) | Factorisation des gabarits (fragments réutilisables) | — |
| 17 | [26.2](17-26.2-multi-tenant.md) | Exécution multi-tenant d'un même pipeline | 25.5, 26.5, 26.4 |

**Pourquoi 26.4 juste avant 26.2** : sans factorisation des gabarits, le multi-tenant multiplie mécaniquement la duplication. Livrer 26.2 seul, c'est livrer le problème à l'échelle N. 26.4 est techniquement autonome et peut donc être avancé à tout moment si une personne est disponible — mais elle ne doit pas passer *après* 26.2.

---

## Chemin critique

```
26.10 ─┬─────────────────────────────────────► 26.1 ──► 26.3
       │                                                 ▲
25.1 ──┼──► 25.5 ─┬──► 26.9 ──► 26.11                    │
       │          │      └──────────────────────────────►┘
25.4 ──┘          │
  └───────────────┼─────────────────────────────────────►┘
                  │
       26.8 ──────┘
                  └──► 26.5 ──┐
                              ├──► 26.2
                     26.4 ────┘

26.6 ──► 26.7   (indépendant, sauf 26.10 pour 26.7)
25.2 ──► 25.3   (indépendant)
25.6            (indépendant)
```

**Le chemin le plus long** est `25.1 → 25.5 → 26.5 → 26.2` (4 stories dont deux structurantes). C'est lui qui détermine la date de disponibilité du multi-tenant.

**Si tu veux paralléliser sur plusieurs personnes** :

- Piste 1 (chemin critique) : 26.10 → 25.1 → 25.4 → 25.5 → 26.5 → 26.2
- Piste 2 (bugs autonomes, démarrable immédiatement) : 25.2 → 25.3 → 25.6 → 26.8
- Piste 3 (webhook, démarrable dès 26.10 livrée) : 26.6 → 26.7
- Piste 4 (topologie, démarrable dès 26.10 livrée) : 26.1 → 26.3

La piste 2 n'a **aucun prérequis** : elle peut démarrer le même jour que la piste 1.

---

## Traçabilité : liste d'origine → story

### Bugs

| Item de la liste | Story | Position |
|---|---|---|
| Pas de state persistance injectable partout dans nos params d'input | 25.5 | 04 |
| `http_call` avec `dataField` sur un tableau de plus d'un élément | 25.3 | 06 |
| `queryParams` accepté par le schéma sur `httpPolling`/`http_call`, implémenté seulement sur `httpRequest` | 25.1 | 02 |
| Output `database` n'implémente pas `PreviewableModule` ⇒ `--dry-run` vide, connexion ouverte | 25.6 | 07 |
| Divergence de portée dans `loop` (`record.record` vs `<alias>`) | 25.4 | 03 |
| `resultKey` accepté par `soap_call`/`sql_call`, refusé par `http_call`, obligatoire sous `append` | 25.2 | 05 |
| Encoder l'URL finale, pas seulement les params | 25.1 | 02 |

### Features

| Item de la liste | Story | Position |
|---|---|---|
| Multi output (condition ?) | 26.1 | 12 |
| Multi tenant | 26.2 | 17 |
| Module split | 26.3 | 13 |
| Agrégation en filter et output | 26.3 | 13 |
| Impossible de factoriser les gabarits (`{% include %}` rejeté) | 26.4 | 16 |
| Flux OAuth `USERNAME_PASSWORD` (ajouter tous les types d'OAuth) | 26.5 | 11 |
| Utilisation d'URL d'input depuis résultat d'OAuth | 26.5 | 11 |
| Réponse du webhook | 26.7 | 15 |
| Plus d'auth d'entrée pour les webhooks (basic, JWT, …) | 26.6 | 14 |
| Routing de webhook au lieu de port unique | 26.6 | 14 |
| Filtre `mapping`/`dateFormat` → option timezone | 26.8 | 08 |
| `Last timestamp = max(LastModifiedDate)` | 26.9 | 09 |
| `skip` ne distingue pas l'échec technique du fonctionnel / marqueur d'erreur | 26.10 | 01 |
| Pagination de type `link` avec `nextUrlField` et/ou `nextUrlHeader` | 26.11 | 10 |

**Factorisations opérées** : 21 items → 17 stories.

| Regroupement | Story | Raison |
|---|---|---|
| `queryParams` ignoré + encodage de l'URL finale | 25.1 | Même cause racine : pas de constructeur d'URL partagé |
| `resultKey`/`mergeStrategy` des 3 filtres d'appel | 25.2 | Un seul contrat à définir |
| Module split + agrégation filter/output | 26.3 | Besoins duaux, même point d'extension : la chaîne de filtres devient N→M |
| Grant types OAuth + URL d'input depuis la réponse du token | 26.5 | Même module `internal/auth`, apparaissent ensemble sur le même connecteur |
| Auth entrante webhook + routing multi-endpoints | 26.6 | Même surface : la porte d'entrée du serveur HTTP |

**Non factorisé volontairement** : la réponse du webhook (26.7) touche le même module que 26.6 mais un autre chemin de code (sortie vs entrée) et pose une question de conception propre (mode ack-immédiat vs synchrone). Les deux restent adjacentes dans l'ordre (14, 15) pour n'ouvrir le handler qu'une fois.

---

## Décisions actées

| Story | Décision |
|---|---|
| 25.2 | **Option A** — `http_call` accepte `resultKey` (défaut `_response`, obligatoire sous `mergeStrategy: append`), contraintes factorisées dans le schéma pour les 3 filtres. Pas de shim de compatibilité. |

## Décisions encore ouvertes

À trancher au démarrage de la story concernée — chacune est signalée dans le fichier :

| Story | Question |
|---|---|
| 25.5 | Garder les raccourcis `timestamp.queryParam` / `id.queryParam` ou les retirer au profit du gabarit seul ? (recommandation : garder) |
| 26.5 | `authorization_code` interactif, ou couvrir `refresh_token` seul avec obtention hors bande ? Où stocker un refresh token rotatif ? |
| 26.6 | Webhook multi-endpoints → un pipeline avec discriminant, ou plusieurs pipelines ? (recommandation : un pipeline) |
| 26.3 | Bornes mémoire de `aggregate` et comportement au dépassement ; ordre garanti ou non après split/aggregate |
| 26.9 | Les records en `skip` comptent-ils dans le `max()` ? (recommandation : non) |
| 26.2 | Périmètre de la source de tenants ; CRON global ou par tenant |

---

## Rappels de définition de « terminé »

Pour chaque story, avant de la considérer close (cf. `cannectors/CLAUDE.md` et `../CLAUDE.md`) :

- `go test ./...` (ou le package ciblé si l'impact est strictement local) — vert
- `golangci-lint run ./...` — 0 issue
- Les exemples touchés sous `examples/` valident avec `./cannectors validate <file> --verbose`
- La documentation correspondante est mise à jour dans `cannectors-doc/` **dans la même session** — règle fondamentale du workspace
- Si un schéma JSON a changé : `pnpm sync-schemas` puis `pnpm check-schemas` dans `cannectors-doc/`
- `pnpm lint`, `pnpm types:check` et `pnpm check-links` dans `cannectors-doc/`
