# Plan d'implémentation — Langage de templating Jinja2 pour les requêtes de pipeline

> Statut : **✅ IMPLÉMENTÉ** (Phases 0-6 terminées). Voir le détail par phase au §7.
> Objectif : remplacer les mécanismes de templating divergents actuels par **un moteur Jinja2 unique** (via `gonja/v2`) avec **échappement contextuel automatique** par cible (URL, JSON, XML) et un **modèle SQL query + parameters bindés**, couvrant les entrées et sorties REST, SOAP et SQL.
>
> **Résultat** : `go test ./...` = 2094 passants, `golangci-lint` = 0 issue, 34/34 exemples validés au binaire, doc `cannectors-doc/` à jour (build production OK). L'ancien moteur regex (`Evaluator`) est entièrement supprimé. Une deuxième passe de revue a été menée après coup : findings et correctifs au §12.

---

## 1. Contexte et objectif

Aujourd'hui, Cannectors templatise déjà du texte dans les requêtes (URL, headers, body, query SQL, enveloppe SOAP), mais via **trois mécanismes indépendants** aux sémantiques différentes, et **sans** stratégie d'échappement cohérente. On veut un vrai langage de templating, expressif et sûr, partagé par tous les chemins d'I/O.

Décisions actées avec le porteur du besoin :

| Décision | Choix retenu |
|---|---|
| **Périmètre** | Unification des moteurs + échappement contextuel, **avec logique à blocs** (`{% if %}` / `{% for %}`) pour assembler des fragments de requête conditionnels ou répéter des blocs. |
| **Moteur** | **Jinja2** via `gonja/v2`, fichiers `.j2` + templates inline. |
| **Syntaxe** | Nouvelle syntaxe Jinja autorisée — pas de contrainte de préserver `{{record.x \| default: "v"}}`. Migration des exemples et de la doc assumée. |
| **SQL** | **Query templatée (texte) + liste `parameters` bindées.** La query Jinja génère le SQL avec placeholders `$N` ; une liste `parameters` (expressions typées contre le record) fournit les valeurs liées. Responsabilité de correction à l'auteur de la config (solution déclarative). |

### Pourquoi Jinja plutôt qu'`expr` (déjà présent) ?

Le projet dépend déjà de `expr-lang/expr` et l'utilise pour `condition:`, `success.expression`, `http_call`, retry hints. Un simple interpolateur `{{ expr }}` aurait couvert la substitution + les fonctions **sans nouvelle dépendance**. Le facteur décisif en faveur de Jinja est le besoin de **logique à blocs dans le texte** (`{% if %}`, `{% for %}`) — assembler une clause `WHERE` optionnelle, répéter un bloc XML SOAP, etc. — qu'`expr` ne couvre pas (ternaire inline uniquement).

Coût accepté : une dépendance supplémentaire et **deux langages d'expression** dans les pipelines (expr pour les conditions de routage/succès, Jinja pour les templates de requête). On atténue en réservant chaque langage à son rôle (cf. §5 : les `parameters` SQL restent en expr pour préserver le typage).

---

## 2. État des lieux (audit du code existant)

### 2.1 Trois mécanismes de templating coexistent

| Mécanisme | Localisation | Syntaxe | Sémantique variable manquante |
|---|---|---|---|
| **Moteur regex `{{record.x}}`** | `internal/template/template.go` | `{{ path \| default: "v" }}` (regex `template.go:34`) | chaîne vide + `WARN` (`template.go:206`) |
| **Parseur SQL maison** | `filter/sql_call.go:384` (`buildParameterizedQuery`), `output/database.go:313` | `{{record.x}}` seulement, **pas de `default`** | `nil` silencieux (`sql_call.go:410`) |
| **Substitution d'état SQL input** | `input/database.go:219+` | `{{lastRunTimestamp}}` + `:namedParam` (manuel `strings.ReplaceAll`) | valeur d'état ou epoch |

À cela s'ajoute une **4ᵉ** substitution, à simple accolade : les placeholders de path `{paramName}` résolus via `strings.ReplaceAll(... url.PathEscape ...)` dans `output/http_request_url.go:67`, `filter/http_call.go:743`, `soaputil/soaputil.go:334`.

### 2.2 Trois (voire quatre) stratégies d'échappement éparpillées

| Cible | Échappement appliqué | Où |
|---|---|---|
| Body JSON (HTTP) | **AUCUN** — substitution brute / éval par feuille sans encodage | `output/http_request.go` `Evaluate()` / `EvaluateMapValues()` |
| URL / query | `url.QueryEscape` | `template.go:164` `EvaluateForURL()` |
| Enveloppe SOAP | `xml.EscapeText` | `soapclient/envelope.go:110` `escapeXMLText()` — **logique hébergée hors de `internal/template`** |
| SQL | paramètres liés (`$1`/`?`) | `sql_call.go:413`, `output/database.go`, `database.FormatPlaceholder` |

> ⚠️ **Risque actif** : un body JSON dont une valeur templatisée contient `"`, `\`, saut de ligne… peut casser le JSON. À corriger via l'échappement contextuel (cf. §4.4).

### 2.3 Capacités actuelles du moteur `internal/template`

- Navigation : dot-notation + index tableau (`record.items[0].name`) via `internal/recordpath/path.go`.
- Accès métadonnées : `{{_metadata.x}}` via `internal/metadata/accessor.go`.
- **Un seul helper** : `| default: "literal"` (`template.go:34`).
- **Pas de** fonctions (upper/lower/trim/date/json…), pas de pipes, pas de conditions, pas d'accès aux variables d'environnement.
- Validation = **équilibrage d'accolades uniquement** (`template.go:245`), au build du module, pas au `validate` de la config.

### 2.4 Inventaire des sites de templating (cibles de migration)

| Module | Champs templatisés | Variante d'éval actuelle |
|---|---|---|
| `output/http_request` | endpoint (URL), headers, body (inline + `bodyTemplateFile`) | `EvaluateForURL`, `Evaluate` |
| `output/soap_request` | endpoint, HTTP headers, body XML, SOAP headers | `soaputil` + `EvaluateXMLTemplate` |
| `output/database` | query SQL | parseur maison → bound params |
| `filter/http_call` | endpoint, headers, body, `cache.key` | idem http_request |
| `filter/soap_call` | endpoint, headers, body, `cache.key` | idem soap_request |
| `filter/sql_call` | query SQL, `cache.key` | parseur maison → bound params |
| `input/http_polling` | endpoint, headers, body (pagination/state) | `EvaluateForURL`, `Evaluate` |
| `input/soap_polling` | endpoint, body XML (pagination/state) | `soaputil` + `EvaluateXMLTemplate` |
| `input/database` | query SQL (`{{lastRunTimestamp}}`, `:named`) | substitution manuelle |

---

## 3. Choix de la librairie

Conformément à la règle « check library first » du projet : pas de réimplémentation maison d'un moteur Jinja.

| Librairie | Fidélité Jinja2 | Maintenance | Remarque |
|---|---|---|---|
| **`github.com/nikolalohinski/gonja/v2`** ✅ *(retenu)* | Élevée — vise la compatibilité Jinja2 (filtres, `{% %}`, `default`, `loop`) | Active | API d'exécution avec contexte (`exec.NewContext`), filtres globaux enregistrables |
| `github.com/flosch/pongo2/v6` | Bonne mais **sémantique Django** (filtres et `default` diffèrent de Jinja2) | Active | Base historique dont dérive gonja — repli si gonja bloque |

**Retenu : `gonja/v2`** pour coller à la sémantique Jinja2 attendue (fichiers `.j2`).

> Ni gonja ni pongo2 n'offrent l'échappement contextuel URL/XML/JSON nativement (leur auto-escape est **HTML, câblé en dur** dans le renderer et non remplaçable via l'API publique). On le câble par-dessus (cf. §4.4) — c'est de toute façon nécessaire avec n'importe quel moteur.

**Footprint transitif** (mesuré au spike, gonja v2.8.0) : tire `sirupsen/logrus`, `json-iterator/go`, `pkg/errors`, `golang.org/x/text`, `golang.org/x/exp`, `dustin/go-humanize`, `modern-go/{reflect2,concurrent}`. `logrus` et `json-iterator` sont les ajouts notables — à valider au regard du critère « taille raisonnable » du projet (§10.7).

---

## 4. Architecture cible

### 4.1 Vue d'ensemble

```
config YAML (template inline ou fichier .j2)
        │
        ▼
internal/template (refonte)               ← façade unique
  ├─ Engine        : wrappe gonja, compile + cache les templates
  ├─ RenderContext : record, meta, state, pagination, env
  └─ Escaper       : stratégie d'échappement selon la cible
        │
        ├── TargetText   → brut (logs, cache.key, texte SQL)
        ├── TargetURL    → url.QueryEscape
        ├── TargetJSON   → encodage JSON-safe des valeurs substituées
        └── TargetXML    → xml.EscapeText (remplace soapclient/envelope.go)
```

> Note : il n'y a **plus de `TargetSQL`**. Avec le modèle « query texte + parameters bindées » (§5), la query est rendue en `TargetText` (texte de confiance, responsabilité auteur) et la sécurité repose sur les paramètres liés par le driver, pas sur l'échappement du template.

### 4.2 Contexte de rendu exposé aux templates

| Variable | Contenu | Remplace l'ancien |
|---|---|---|
| `record` | le record courant (`map[string]any`) | `{{record.x}}` |
| `meta` | métadonnées (`_metadata`) via `metadata.Accessor` | `{{_metadata.x}}` |
| `state` | état de persistance (timestamp, id, offset) | `{{lastRunTimestamp}}` |
| `pagination` | page / offset / cursor courant | injecté ad hoc aujourd'hui |

> `env` (variables d'environnement en lecture seule) était prévu au plan initial : **non retenu**. Le champ existait dans `RenderContext` sans qu'aucun appelant ne le remplisse (`env.*` rendait donc toujours vide) ; il a été supprimé plutôt que laissé en promesse morte. La résolution `${VAR}` des configs couvre déjà le besoin en amont du templating. À réintroduire seulement avec un vrai mécanisme d'allow-list explicite.

Exemples de syntaxe cible :

```jinja
{{ record.user.id }}
{{ record.items[0].name | default("n/a") }}
{{ record.email | lower }}
{% if record.priority == "high" %}URGENT{% endif %}
{% for tag in record.tags %}<tag>{{ tag }}</tag>{% endfor %}
```

> Migration : `| default: "v"` → filtre Jinja `| default("v")`. Le préfixe `record.` devient une **vraie variable de contexte**.

### 4.3 Sémantique « variable manquante »

gonja expose nativement `config.Config.StrictUndefined` (**confirmé au spike** ; défaut `false`). Choix proposé :
- **Strict (`StrictUndefined: true`, erreur)** pour SOAP et SQL — cohérent avec l'actuel SOAP (`envelope.go:105` échoue déjà si variable absente sans default).
- **Tolérant (`false`, rend une chaîne vide)** + `default()` pour REST (URL/headers/body) — cohérent avec l'actuel (chaîne vide + WARN), évite de casser des pipelines sur un champ optionnel.

Implémentation : un `config.Config` par cible, donc on construit nos templates via `exec.NewTemplate(id, cfg, loader, env)` plutôt que le `gonja.FromString` de convenance (qui force le `DefaultConfig`). À acter définitivement en Phase 1.

### 4.4 Échappement contextuel — l'unification clé

> **Mécanisme validé au spike Phase 0.** L'autoescape HTML de gonja n'étant pas remplaçable, l'échappement par cible est obtenu en **deux temps** :
> 1. Enregistrer des **filtres d'échappement custom** dans l'`Environment` (`FilterSet.Register`) : `__urlesc` (`url.QueryEscape`), `__xmlesc` (`xml.EscapeText`), `__jsonesc` (échappement de chaîne JSON via `json.Marshal` puis retrait des guillemets encadrants). Ils renvoient `exec.AsSafeValue(...)` pour éviter tout double-échappement.
> 2. À la compilation, **réécrire chaque balise de sortie** `{{ expr }}` → `{{ (expr) | __<cible>esc }}` selon la cible du champ. Le texte littéral et les structures `{% %}` ne sont pas touchés → seuls les valeurs substituées sont échappées, jamais la structure (URL/XML/JSON) écrite par l'auteur.
>
> ⚠️ La réécriture doit utiliser un **scanner de balises robuste** (respect de `{% %}`, `{# #}`, littéraux de chaîne contenant `}}`), **pas une simple regex** comme dans le POC. C'est le principal travail de durcissement de la Phase 1.

Chaque site d'appel déclare sa **cible** ; le moteur applique l'échappement adapté. On supprime `EvaluateForURL` / `EvaluateXMLTemplate` au profit d'un point d'entrée unique paramétré.

```go
type Target int
const (
    TargetText Target = iota // cache.key, logs, texte SQL : aucun échappement
    TargetURL                // endpoint, query params : url.QueryEscape
    TargetJSON               // body JSON : valeurs encodées JSON-safe
    TargetXML                // enveloppe/headers SOAP : xml.EscapeText
)

func (e *Engine) Render(tmpl *Compiled, ctx RenderContext, target Target) (string, error)
```

**Body JSON — deux formes supportées :**
- **Objet structuré** (body déclaré comme map/array YAML, cas actuel d'`EvaluateMapValues`) : on **évalue chaque feuille** ; les feuilles non-string conservent leur type, les feuilles string sont rendues puis ré-encodées JSON-safe. La structure JSON ne peut pas être cassée. **Forme recommandée.**
- **Template texte / fichier `.j2`** : rendu texte avec échappement JSON-safe des valeurs substituées.

---

## 5. Modèle SQL : query templatée + paramètres bindés

C'est le modèle standard d'une requête paramétrée, et il **fait disparaître** l'ancien point dur (interception des nœuds de sortie SQL).

### 5.1 Forme de configuration cible

```yaml
sql_call:
  query: |
    SELECT * FROM orders
    WHERE customer_id = $1
    {% if record.status %}AND status = $2{% endif %}
  parameters:
    - record.customer.id      # → $1
    - record.status           # → $2
```

- **`query`** : template **Jinja** rendu en **texte** (`TargetText`). Peut utiliser `{% if %}` / `{% for %}` pour la *forme* de la requête. Les placeholders `$N` y figurent **littéralement**.
- **`parameters`** : liste ordonnée, chaque entrée est une **expression `expr`** évaluée contre le record → **valeur native typée** liée au placeholder `$N` correspondant.

### 5.2 Pourquoi `parameters` en `expr` et non en Jinja

Jinja rend du **texte** ; or un paramètre lié doit préserver son **type** (int, float, bool, time, `nil`→`NULL`). Passer `"123"` (string) là où la colonne est un entier est lossy, et `nil` doit devenir `NULL`, pas `""`. L'actuel parseur maison passe déjà la valeur typée de `recordpath.Get` directement. On préserve cette correction en évaluant chaque `parameters[i]` via **`expr`** (déjà la lib du projet, type-safe) plutôt que par rendu texte Jinja.

> Conséquence assumée : dans un bloc SQL, `query` est en Jinja et `parameters` en expr. Séparation par rôle (forme texte vs valeur typée), cohérente avec l'usage existant d'expr.

### 5.3 Traduction des placeholders selon le driver

L'auteur écrit `$1`, `$2`… (canonique). La cible SQL normalise selon le driver via `database.FormatPlaceholder` :
- **postgres** : `$1` tel quel, réutilisation possible (`$1` cité 2× → 1 seul arg).
- **mysql / sqlite** : `$N` → `?` positionnel ; si un `$N` est réutilisé, la valeur est ré-émise dans l'ordre.

Cette logique driver-specific existe déjà dans `input/database.go` ; on la centralise dans le normaliseur SQL partagé.

### 5.4 Garde-fous

- **Cohérence placeholders/params** : valider à la construction que tous les `$N` référencés ont un `parameters[N-1]`, et inversement (pas de param orphelin). Erreur explicite sinon.
- **Aucune donnée hors paramètre — imposé, pas seulement documenté** : depuis l'audit final, `sqltemplate.Compile` **rejette toute balise de sortie `{{ … }}` dans la query** (`template.HasOutputTag`). L'interpolation d'une valeur de record dans le texte SQL (vecteur d'injection) est donc impossible ; seuls les blocs `{% if %}`/`{% for %}` sont autorisés pour façonner la requête. C'est vérifié dès `cannectors validate`.
- **Scan des placeholders conscient des literals** : `$N` à l'intérieur d'une chaîne SQL (`'…'` avec échappement `''`), d'un dollar-quote (`$$…$$` / `$tag$…$tag$`) ou d'un commentaire (`--`, `/* */`) n'est **pas** traité comme placeholder (ni compté, ni traduit, ni bindé). Remplace la regex `\$[0-9]+` naïve qui corrompait `'coût $5'` ou décalait les arguments en mysql.
- Contrôle « aucun délimiteur `{{`/`}}` résiduel après rendu » conservé comme filet fail-closed (une balise résiduelle = bug du tokenizer → on refuse la requête).

### 5.5 SQL input

`input/database.go` : `{{lastRunTimestamp}}` devient `{{ state.lastRunTimestamp }}` dans la `query`, et les `:named` deviennent des entrées `parameters` en expr (`state.lastRunId`, etc.). Suppression de la substitution manuelle `strings.ReplaceAll`.

---

## 6. Fichiers `.j2` et templates inline

- **Inline** : la valeur YAML (`endpoint`, `body`, `query`, header values) est un template Jinja compilé tel quel.
- **Fichier** : généraliser le mécanisme existant `bodyTemplateFile` (`http_request.go:220`) à toutes les cibles, convention d'extension `.j2` :
  - `bodyTemplateFile` (existant) — body REST/SOAP.
  - `queryFile` — query SQL externe.
  - Résolution de chemin via `internal/pathutil`.
- Chargement + compilation **une fois** au build du module, mis en cache. Le cache non-thread-safe de l'actuel `Evaluator` est remplacé par un cache de templates **compilés** (un `*Compiled` gonja réutilisable et sûr en lecture concurrente).

---

## 7. Plan d'implémentation par étapes

> Chaque étape se termine par `go test ./...` + `golangci-lint run ./...` au vert (règle projet). La doc `cannectors-doc/` est mise à jour dans la même session (règle workspace).

### Phase 0 — Spike de faisabilité ✅ FAIT

Réalisé en module isolé (`gonja/v2 v2.8.0`, `go 1.26`). Résultats :

| Point validé | Verdict |
|---|---|
| Contexte `record`, accès imbriqué + index tableau (`record.items[0].name`) | ✅ |
| Blocs `{% if %}…{% else %}…{% endif %}` et `{% for … %}` | ✅ |
| Filtre `default("…")` | ✅ |
| Échappement **URL** auto (filtre custom + réécriture des balises) | ✅ `?q={{record.q}}` → `?q=a+b%26c`, littéraux préservés |
| Échappement **XML** auto | ✅ `<>&"` correctement échappés, structure intacte |
| Échappement **JSON** auto | ✅ JSON valide et re-parsable (après correction du découpage des guillemets) |
| Sémantique variable manquante | ✅ `config.StrictUndefined` natif (lenient par défaut) |
| Cache de templates compilés | ✅ **~6× plus rapide** : 13.5 µs (compile à chaque fois) → **2.2 µs/rendu** (compilé caché), 44 allocs |

**Conclusions** : gonja est viable ; le mécanisme d'échappement par filtres custom + réécriture des balises fonctionne pour les 3 cibles ; le strict-undefined et les blocs sont natifs ; le cache de templates compilés est indispensable et efficace. **Seul point de durcissement reporté** : remplacer la regex naïve du POC par un scanner de balises robuste (§4.4).

### Phase 1 — Nouveau cœur `internal/template` ✅ FAIT

Livré, **en coexistence** avec l'ancien `Evaluator` (toujours utilisé par les modules jusqu'aux Phases 2-5 — le repo compile). Nouveaux fichiers dans `internal/template/` :

| Fichier | Contenu |
|---|---|
| `engine.go` | `Engine` (cache `sync.Map`, sûr en concurrence), `Target` (`Text/URL/JSON/XML`), `Compiled.Render`, `Engine.Compile/Validate`. Config gonja fraîche par compile (évite la fuite de `{% autoescape %}`). |
| `context.go` | `RenderContext` → variables `record` / `meta` / `state` / `pagination` / `env` (toujours présentes, vides si absentes). |
| `escape.go` | Filtres d'échappement `__cnx_urlesc` / `__cnx_xmlesc` / `__cnx_jsonesc` (renvoient `AsSafeValue`), enregistrés une fois (`sync.Once`) ; logs gonja coupés. |
| `rewrite.go` | `injectEscape` — **scanner de balises robuste** (respecte `{% %}`, `{# #}`, littéraux de chaîne contenant `}}`, marqueurs `{{- -}}`). Remplace la regex naïve du POC. |
| `engine_test.go` | 25 cas : navigation/index, filtres (`upper/capitalize/default/join/length`), blocs `if`/`for`, cibles URL/XML/JSON (injection), `TargetText` sans échappement, strict vs lenient, cache, syntaxe invalide, unitaires du scanner. |

**Raffinement de conception** vs plan initial : la **strictness est un paramètre séparé** (`Compile(src, target, strict)`), **non liée à la cible**. Raison : `TargetText` est partagée par le texte SQL (strict souhaité) et `cache.key` (lenient) — la cible seule ne pouvait pas porter la strictness.

`ValueToString` de l'ancien moteur est conservé tel quel (réutilisable, pas de re-port). Branchement de `Validate()` dans `cannectors validate` : reporté en Phase 5 comme prévu (la méthode est exposée).

→ **verify atteint** : `go test ./...` = **2171 passants** ; `golangci-lint run ./...` = **0 issue**.

### Phase 2 — Migration HTTP (REST) ✅ FAIT

**Périmètre réel** : `output/http_request` + `filter/http_call`. `input/http_polling` **ne templatise rien aujourd'hui** (aucun appel au moteur) — rien à migrer ; un templating d'endpoint/body en entrée serait une fonctionnalité nouvelle, hors Phase 2.

Réalisé :
1. Les deux modules branchés sur l'`Engine` (champ `engine *template.Engine`, helper `renderField(src, target, record)` + `recordRenderContext` exposant `record` + `meta`). Compile-on-demand caché → sites d'appel quasi inchangés.
2. Cibles : endpoint→`TargetURL`, headers→`TargetText`, body→`TargetJSON` (si Content-Type JSON, sinon `TargetText`), `cache.key`→`TargetText`. Mode lenient (REST).
3. `EvaluateForURL` / `template.ValidateSyntax` retirés de ces modules (validation = `engine.Validate(..., target, false)` au build). `EvaluateMapValues`/`EvaluateHeaders` n'étaient pas utilisés.
4. **Défaut JSON corrigé** : valeurs substituées encodées JSON-safe. Test de régression module ajouté (`TestHTTPRequestModule_Templating_JSONBodyEscaping` : un record avec `"`, `\`, `\n` round-trippe, body JSON valide).

**Migrations de syntaxe induites** : `| default: "v"` → `| default('v')` (exemple 19 + assets `geocode_request.json`, `task_update.json`). Un `}}` orphelin n'est plus une erreur (gonja → texte littéral, cohérent avec un body JSON imbriqué) ; sous-test `unmatched closing brace` retiré.

→ **verify atteint** : `go test ./...` = **2171 passants** ; `golangci-lint run ./...` = **0 issue** ; exemples 16 & 19 revalidés via `TestExampleConfigsAreSchemaCompliant`.

### Phase 3 — Migration SOAP ✅ FAIT

Réalisé :
1. `soapclient/envelope.go` : `EvaluateXMLTemplate` ré-implémenté sur l'`Engine` (engine partagé `xmlTemplateEngine`, `TargetXML`, **strict**). Suppression de `resolveTemplateVariable`. `body`/SOAP headers évalués via cette voie (appelée par `client.go`).
2. `soaputil/soaputil.go` : engine partagé `templateEngine` + helper `renderTemplate`. `ValidateBase` valide par cible (endpoint→URL, body/headers SOAP→XML, HTTP headers / MTOM ContentID→Text). `BuildOperation` (endpoint→URL, HTTP headers→Text), `BuildMTOMConfig` (ContentID→Text), `BuildEndpointWithKeys` (endpoint→URL) migrés (lenient).
3. `filter/soap_call.go` : `engine` au lieu de `templateEvaluator` ; `cache.key`→`TargetText` ; validation par compilation. `soap_request`/`soap_polling` ne templatisent pas directement (délèguent à soaputil/soapclient) — rien à y changer.

**Déviation assumée vs plan** : `escapeXMLText` est **conservé** dans `soapclient` — il n'est pas lié au templating, il est utilisé par `security.go` pour l'échappement XML direct des champs WS-Security (username/password/created). Seul le *templating* d'`EvaluateXMLTemplate` a migré ; l'échappement des valeurs substituées vit désormais dans l'`Escaper` (`TargetXML`).

**Découverte technique importante** (corrigée) : le filtre d'échappement auto-injecté **avalait l'erreur strict-undefined** — en mode strict une variable manquante devient une `Value` d'erreur, que mes filtres stringifiaient (`<errors.fundamental Value>`) au lieu de la propager. Fix : chaque filtre `__cnx_*esc` court-circuite sur `in.IsError()` et retourne l'erreur. Test de régression ajouté (`TestEngine_StrictUndefined_WithEscapeFilter`). Sans ce fix, un body SOAP avec champ manquant aurait été envoyé silencieusement au lieu d'échouer.

**Migration de syntaxe** : `| default: ""` → `| default('')` dans les bodies SOAP des exemples 41 et 43.

→ **verify atteint** : `go test ./...` = **2172 passants** (dont le test d'injection XML `envelope_test.go`) ; `golangci-lint run ./...` = **0 issue** ; exemples 41 & 43 revalidés.

### Phase 4 — Migration SQL (modèle query + parameters) ✅ FAIT

Réalisé :
1. **Nouveau package `internal/sqltemplate`** (15 tests) : `Compile(engine, querySrc, paramSrcs, driver)` + `(*Query).Build(rc)`. La query est un template Jinja rendu en texte (`TargetText`, **strict**) ; les `parameters` sont des expressions **expr** (`AllowUndefinedVariables` — pattern maison du projet) évaluées **paresseusement** : un `$N` dans une branche `{% if %}` non prise n'est ni évalué ni bindé. Traduction driver : postgres renumérote `$N` par première apparition (réutilisation d'un arg pour les répétitions), mysql/sqlite émettent un `?` + un arg par occurrence. Garde-fous : validation statique `$N`↔params à la compilation (placeholder au-delà des params déclarés / param jamais référencé = erreur), délimiteurs `{{ }}` résiduels après rendu = erreur (héritier de `sql_call.go:420`).
2. **3 modules migrés**, `buildParameterizedQuery` maison supprimés des deux côtés :
   - `filter/sql_call` : query+parameters (contexte `record`/`meta`), `cache.key`→`TargetText` lenient.
   - `output/database` : idem, `getDBFieldValue` supprimé.
   - `input/database` : suppression totale de `{{lastRunTimestamp}}`, des `:named` params, du champ `Parameters map[string]any`, de `Incremental.TimestampParam/IDParam` et de `Pagination.Param`. Nouveau contexte : `state.lastRunTimestamp` (RFC3339, epoch au 1er run) / `state.lastRunId`, et `pagination.offset|cursor|limit` **réévalués par page**. Si aucun parameter ne référence `pagination`, le module append `LIMIT/OFFSET` littéral (comportement historique conservé).
3. **Schémas migrés** : `sqlRequestBase` gagne `parameters` (array de strings) + nouvelles descriptions ; `databasePaginationConfig` perd `param` ; `databaseIncrementalConfig` perd `timestampParam`/`idParam` ; le `parameters` object legacy de l'input database supprimé. Contract tests (`sql_contract_test.go`) mis à jour — les champs legacy sont **rejetés** par le schéma.
4. **Exemples migrés et validés au binaire** (`./cannectors validate`) : 06, 08, 09, 17, 18, 21 + assets `customer_lookup.sql`, `upsert_product.sql`. L'exemple 09 illustre le gain d'expressivité : `pagination.cursor ?? state.lastRunId ?? 0` remplace l'ancien partage ambigu du placeholder `:last_id` entre incremental et pagination.

**Nuance vs plan initial (§4.3)** : le « strict » SQL s'applique à la **query** (la forme — variable Jinja manquante = erreur) ; les **parameters** expr sont permissifs (champ manquant → `nil` → `NULL`), fidèle à l'ancien comportement SQL et au pattern expr du projet ; l'auteur peut opter pour l'optionnel explicite via `?.` / `??`.

→ **verify atteint** : `go test ./...` = **2180 passants** (dont typage int/float/bool préservé, nil→NULL, clauses conditionnelles, renumérotation postgres vs ré-émission mysql, cohérence `$N`/params) ; `golangci-lint run ./...` = **0 issue** ; 6 exemples SQL validés au binaire.

### Phase 5 — `cache.key`, validation config, nettoyage ✅ FAIT

Réalisé :
1. `cache.key` → `TargetText` : déjà couvert au fil des Phases 2-4 (http_call, sql_call, soap_call) — vérifié, rien à reprendre.
2. **`cannectors validate` compile désormais les templates** (`internal/config/template_validation.go`, branché dans `runValidate` après la validation schéma). Deux passes **sans I/O réseau ni construction de modules** (donc sans faux positifs liés aux env vars/connexions) :
   - *Générique* : toute valeur string contenant `{{` ou `{%` est compilée (syntaxe Jinja) ; erreur rapportée avec le path JSON (`/output/endpoint`, etc.).
   - *SQL* : pour chaque module `database`/`sql_call`, compilation `query`+`parameters` via `sqltemplate` → cohérence `$N`/params et syntaxe expr vérifiées. `queryFile` lu en best-effort (chemin relatif au cwd) ; illisible → ignoré ici, la construction du module le rapportera.
   - Vérifié e2e : endpoint Jinja cassé → `exit=1` avec path exact ; `$2` sans 2ᵉ parameter → `exit=1` ; les 34 exemples → `exit=0`.
3. **Ancien moteur supprimé** : `Evaluator`, `NewEvaluator`, `Evaluate`, `EvaluateForURL`, `EvaluateHeaders`, `EvaluateMapValues`, `ParseVariables`, `ValidateSyntax`, `Variable` et la regex n'existent plus. `template.go` ne conserve que `HasVariables` et `ValueToString` (encore utilisés comme fast-path/conversion). **Fix au passage** : `HasVariables` détecte désormais aussi `{%` — un template composé uniquement d'un bloc (`{% if %}…{% endif %}` sans `{{ }}`) aurait été ignoré par les fast-paths des modules.
4. **Décision `{paramName}` (§10.2) : conservé, documenté comme mécanisme distinct.** Les placeholders à simple accolade appartiennent au système `keys` (`field`/`paramType`/`paramName`) — une liaison record→requête structurée avec `url.PathEscape` propre, orthogonale au templating. Les unifier sous Jinja casserait la sémantique des keys header/query/path sans bénéfice. La doc (Phase 6) devra expliciter la distinction `{param}` (keys) vs `{{ expr }}` (templates).

→ **verify atteint** : `go test ./...` = **2184 passants** ; `golangci-lint run ./...` = **0 issue** ; **34/34 exemples** validés au binaire.

### Phase 6 — Schémas, exemples, documentation ✅ FAIT

Points 1 (schémas) et 2 (exemples) déjà réalisés au fil des Phases 4-5 (migrés + validés au binaire). Reste la doc `cannectors-doc/`, faite ici :
1. **Schémas resynchronisés** : `pnpm sync-schemas ../cannectors` (les vendored `schemas/cannectors/*.json` + les 34 exemples vendorés reflètent le nouveau modèle) ; `pnpm check-schemas` cohérent ; `pnpm generate-subpages` (sous-pages `pagination`/`incremental` régénérées depuis les schémas).
2. **Nouvelle page cœur `concepts/templating.mdx`** (ajoutée à la sidebar après `records`) : syntaxe Jinja (tags/filtres/blocs), table des contextes (`record`/`meta`/`state`/`pagination`), échappement contextuel par cible, modèle SQL `query`+`parameters` (expr, typage, `??`/`?.`, traduction `$N`→driver, validation `$N`/params), sémantique strict/lenient, et la distinction explicite `{paramName}` (keys) vs `{{ expr }}` (templates).
3. **Pages migrées** : `concepts/records.mdx` (section Templates réécrite + lien), `modules/filters/sql-call`, `modules/outputs/database`, `modules/inputs/database` (pagination `pagination.offset/cursor/limit` + incremental `state.lastRun*`, suppression de `param`/`:named`/`timestampParam`), `concepts/soap.mdx` + `modules/inputs/soap-polling` (`default('')`), `concepts/state-persistence.mdx` (`$1` + `parameters: [state.lastRunTimestamp]`), `modules/outputs/http-request` (échappement JSON, filtres), `cli/validate.mdx` (compilation des templates). Plus aucune trace de `| default: "…"`, `{{record.x}}` SQL, `:namedParam`, `timestampParam`/`idParam`/`pagination.param` dans la doc.
   → **verify atteint** : ESLint 0 issue ; `pnpm types:check` OK ; `pnpm check-links` (211 liens) OK ; **`pnpm build` production réussi**.

---

## 8. Impact par module (matrice de migration)

| | REST | SOAP | SQL |
|---|---|---|---|
| **Input** | http_polling : URL/headers/body | soap_polling : URL/body XML | database : query + parameters (`state.*`) |
| **Filter** | http_call : URL/headers/body/cache.key | soap_call : URL/body/cache.key | sql_call : query + parameters/cache.key |
| **Output** | http_request : URL/headers/body | soap_request : URL/body XML | database : query + parameters |

Cible d'échappement par champ : **endpoint→URL**, **headers→Text**, **body REST→JSON**, **body SOAP→XML**, **query SQL→Text + params bindés**, **cache.key→Text**.

---

## 9. Sécurité

| Cible | Garantie après refonte |
|---|---|
| **SQL** | Valeurs dynamiques via `parameters` → paramètres liés typés (nil→NULL). Validation cohérence `$N`/params. Interpolation directe dans `query` déconseillée et documentée. |
| **XML/SOAP** | `xml.EscapeText` centralisé (plus de duplication dans `soapclient`). |
| **URL** | `url.QueryEscape` sur les valeurs de variables. |
| **JSON** | **Nouveau** : valeurs encodées JSON-safe → corrige le risque de casse/injection du body actuel. |
| **Chargement de templates** | `{% include %}` / `{% extends %}` / `{% import %}` / `{% from %}` **rejetés à la compilation** + loader gonja qui refuse toute résolution : un template chargé depuis le disque échapperait à la réécriture d'échappement, et son chemin peut être une expression alimentée par le record. |

---

## 10. Risques et points ouverts

1. **Filtre d'échappement auto par cible dans gonja** — ✅ **résolu au spike** (filtres custom + réécriture des balises, §4.4). Reste un travail de durcissement (scanner de balises robuste, pas une regex) à mener en Phase 1.
2. **Placeholders `{paramName}` à simple accolade** — ✅ **tranché en Phase 5** : conservés comme mécanisme distinct lié aux `keys` (liaison structurée record→requête, orthogonale au templating). À documenter clairement en Phase 6.
3. **Deux langages d'expression** (expr pour conditions/params SQL, Jinja pour templates) — accepté ; à clarifier dans la doc pour éviter la confusion utilisateur.
4. **Sémantique `Undefined`** (§4.3) — strict SQL/SOAP, tolérant REST. À acter en Phase 1.
5. **Volume de migration** — 40+ exemples + doc. Prévoir une revue exemple par exemple.
6. **Performance** — ✅ **mesuré au spike** : 2.2 µs/rendu avec template compilé caché (vs 13.5 µs sans). Le cache est donc indispensable mais le coût par record est négligeable. Confirmer sur un pipeline réel à fort volume.
7. **Taille binaire / dépendances** — gonja v2.8.0 tire `logrus`, `json-iterator`, `pkg/errors`, `x/text`, `x/exp`, `go-humanize`, `modern-go/*` (cf. §3). À arbitrer au regard du critère « taille raisonnable ».

---

## 11. Critères de succès globaux

- Un **seul** point d'entrée de templating dans `internal/template`, paramétré par `Target` ; plus aucun parseur SQL maison ni échappement dupliqué.
- Tous les chemins REST/SOAP/SQL (input/filter/output) passent par ce moteur.
- Échappement correct et testé pour chaque cible, **dont le JSON** (régression de sécurité corrigée).
- SQL : query Jinja (avec blocs) + `parameters` typés bindés ; cohérence `$N`/params validée.
- Fonctions/filtres/blocs Jinja disponibles partout, sans code helper maison.
- `go test ./...` + `golangci-lint run ./...` au vert ; exemples revalidés ; `cannectors-doc/` à jour.

---

## 12. Audit de sécurité final

Relecture adversariale du templating et du modèle SQL (post-implémentation). Findings et résolution :

| # | Sévérité | Finding | Statut |
|---|---|---|---|
| C1 | **Critique** | **Bypass d'échappement** : `findTagClose` appliquait le suivi de quotes aux commentaires `{# … #}`, alors que gonja ferme un commentaire au 1er `#}`. Une apostrophe dans un commentaire (`{# don't #}`) faisait diverger le tokenizer → les `{{ }}` suivants **rendus non échappés** (injection XML/JSON/URL via données record). Reproduit : `record.name = "</x><evil/>"` ressortait brut. | ✅ **Corrigé** : tokenizer unique `scanTemplate` (source unique partagée par `injectEscape` et `HasOutputTag`) ; commentaires fermés au 1er `#}` ; blocs `{% raw %}` émis verbatim. Tests de régression `TestEngine_CommentDoesNotBypassEscaping`, `TestEngine_RawBlockNotRewritten`. |
| E2 | **Élevée** | **Injection SQL par interpolation** : `{{ record.x }}` dans une query rendait la valeur en texte brut ; le check « délimiteurs résiduels » ne l'attrapait pas. La sécurité reposait sur la discipline de l'auteur. | ✅ **Corrigé** : `sqltemplate.Compile` **rejette toute balise `{{ }}`** dans la query (`HasOutputTag`) ; vérifié dès `validate`. L'injection par interpolation est désormais **impossible**. |
| E1 | **Élevée** | **Regex `$N` naïve** : `$1` dans un literal (`'coût $1'`), un dollar-quote (`$$…$1…$$`) ou un commentaire était traité comme placeholder → rejet de requêtes valides, ou en mysql corruption + décalage des arguments bindés. | ✅ **Corrigé** : `scanPlaceholders` s'appuie sur **un seul regexp d'alternation** (`sqlToken`, RE2 stdlib, temps linéaire) qui consomme d'abord les chaînes `'…'` (échappement `''`), commentaires `--`/`/* */` et dollar-quotes `$$…$$`, et ne capture `$N` qu'en dehors. Tests `TestPlaceholders_SkipLiteralsAndComments`. *Edge documenté* : les dollar-quotes **tagués** `$fn$…$fn$` (rares, nécessiteraient une backreference que RE2 n'a pas) ne sont pas gérés. |
| M2 | Moyenne | `url.QueryEscape` encode l'espace en `+` (sémantique query), incorrect pour un segment de **path**. | ⚠️ **Assumé** (comportement pré-existant) : sur-encoder est plus sûr que sous-encoder (`&`,`=`,`/` restent encodés) ; `PathEscape` laisserait passer `&`/`=` dans les query params. Compromis défendable, documenté. |
| M3 | Moyenne | Double-échappement si l'auteur pré-encode explicitement (`{{ x \| urlencode }}`). | ⚠️ **Assumé** : sur-échappement (jamais sous-échappement) ; pas d'opt-out `\| safe` volontairement (ajouter un mécanisme de bypass irait contre l'objectif sécurité). Documenté : ne pas pré-encoder. |
| F1 | Faible | `xml.EscapeText` n'échappe pas `]]>` (contexte CDATA non prévu) ; `nil`/objets rendus via `String()` gonja. | ⚠️ **Assumé** : l'échappement cible texte/attribut XML ; CDATA hors périmètre. |

### Deuxième passe de revue (post-audit) — findings et correctifs

Relecture externe du diff complet. Tout ce qui suit est corrigé et couvert par un test de régression.

| # | Sévérité | Finding | Correctif |
|---|---|---|---|
| C2 | **Critique** | **`{% include %}` contournait l'échappement ET la garantie SQL.** Le contenu chargé par gonja n'est pas réécrit par `injectEscape` → toutes ses balises sortaient brutes (reproduit : un `.j2` inclus rendait `<a><evil/></a>` au lieu des entités). Et comme le chemin est une expression, `{% include record.p %}` dans une query passait `HasOutputTag` : lecture de fichier arbitraire injectée verbatim dans le texte SQL. | Rejet à la compilation de `include`/`extends`/`import`/`from` (`checkBlocks`, message explicite avec le path au `validate`) + `denyLoader` en backstop fail-closed à la place du `FileSystemLoader` rooté sur le cwd. Tests `TestEngine_TemplateLoadingBlocksRejected`, `TestCompile_RejectsTemplateInclusion`. |
| M4 | Moyenne | **Régression SOAP sur valeur `null`.** L'ancien moteur échouait si la variable était absente **ou nulle** ; `StrictUndefined` ne couvre que la clé absente, donc un champ présent-mais-`null` partait en élément vide. | Variantes strictes des filtres d'échappement (`__cnx_*esc_strict`) qui rejettent `nil`. `| default('…')` reste opérant (le `default` de gonja substitue sur `nil`). Test `TestEngine_StrictRejectsNullValue`. |
| M5 | Moyenne | **`env` documenté mais mort** (aucun appelant ne remplissait `RenderContext.Env`). | Champ supprimé (cf. §4.2) ; `renderEnv` de `sqltemplate` remplacé par l'unique `RenderContext.Vars()` (fin de la duplication). |
| M6 | Moyenne | **`validate` sautait silencieusement les `queryFile`** : fichier illisible → `return` sans erreur, donc les modules SQL à query externe n'avaient jamais le contrôle `$N`↔`parameters`. | Erreur de validation explicite (`/output/queryFile`) ; la résolution reste relative au cwd, comme `loadQueryFromFile` au runtime. Test `TestValidateTemplates_QueryFile`. |
| F2 | Faible | **`UsesPagination()` en `strings.Contains(src, "pagination")`** : `record.paginationToken` en faux positif → le module n'ajoutait plus `LIMIT/OFFSET` et la boucle de pagination rejouait la même page. | Inspection de l'AST expr (`parser.Parse` + `ast.Walk`, identifiant racine). Test `TestUsesPagination_NoSubstringFalsePositive`. |
| F3 | Faible | **`{% filter upper %}` corrompt l'échappement** (`&lt;` → `&LT;`, XML invalide) : le bloc transforme après l'échappement injecté. | Rejeté pour les cibles échappées (URL/JSON/XML), autorisé en `TargetText`. Test `TestEngine_FilterBlockRejectedForEscapedTargets`. |
| F4 | Faible | **`Content-Type` templatisé** → `bodyTarget` calculé au build depuis les headers statiques ratait l'échappement JSON. | `bodyTargetFor(record)` résout le media type par record (fallback JSON = sur-échappement plutôt que payload cassé) ; la validation JSON post-rendu suit la même cible. |
| F5 | Faible | **Fichier parasite `internal/modules/output/sqlite::memory:`** créé par un test (`connectionString: "sqlite::memory:"`). | DSN corrigé en `:memory:`, fichier supprimé. |
| F6 | nit | Divergence du tokenizer sur `\` dans un littéral de chaîne (notre scanner l'interprète comme échappement, gonja refuse le backslash) : fail-closed aujourd'hui, mais non testé. | Test `TestEngine_BackslashInStringLiteralFailsClosed` qui fige le comportement. |
| F7 | nit | Un `template.NewEngine()` par instance de module → caches de compilation non partagés. | Engine partagé par package (`filter`/`input`/`output`, comme `soaputil`). |

→ **verify atteint** : `go test ./...` = **2094 passants** ; `golangci-lint run ./...` = **0 issue** ; **34/34 exemples** validés au binaire ; rejet vérifié e2e au `validate` (`/output/body` et `/output/query` sur un `{% include %}`).

**Vérifié sans problème** (garde-fous suffisants) : concurrence de `Query.Build` (état local par appel, `vm.Run`/gonja `Execute` sans état partagé mutable), alignement placeholder↔arg (renumérotation postgres / ré-émission mysql), absence de panic (`enc[1:len(enc)-1]` gardé, indexations bornées).

### Passe de propreté (post-audit)

- **Scanner SQL → regexp d'alternation** : le mini-lexer octet-par-octet de `scanPlaceholders` (~110 lignes de helpers `isIdentChar`/`skip*`) est remplacé par un unique `regexp` stdlib (`sqlToken`, RE2, temps linéaire) — plus lisible, même comportement (tests inchangés), edge tagged-dollar-quote documenté.
- **Code mort supprimé (correction d'un oubli Phase 4)** : la Phase 4 avait orphané `database.ConvertPlaceholders`, `database.FormatPlaceholder` et leurs helpers `skipQuoted`/`skipLineComment`/`skipBlockComment` (plus aucun appelant après le retrait des `buildParameterizedQuery` maison) — l'affirmation « aucun code mort » de la première version de cet audit était donc **inexacte**. Cluster + tests supprimés. `GetPlaceholderStyle` (toujours utilisé par `sqltemplate`) conservé.
- **Deuxième passe outillée (whole-program)** : le grep manuel ratant les symboles **exportés** inutilisés, la 2ᵉ passe utilise `golang.org/x/tools/cmd/deadcode` (reachability depuis `main`). Dans le périmètre templating : `getRecordFieldString` **supprimé**. 66 fonctions internes restaient signalées, mais `deadcode -test` (tests comme racines) a montré qu'**une seule** (`RetryExecutor.LastErrors`) était morte partout — les 65 autres sont exercées par la suite de tests.
- **Troisième passe — nettoyage complet du repo** (demandé explicitement). Critère de suppression : symbole inatteignable depuis `main` **ET** dont les seules références sont son propre test dédié (feature auto-testée jamais câblée). Critère de conservation : tout ce qui sert d'**infrastructure de test partagée** (helpers utilisés par plusieurs cas / d'autres packages), les méthodes d'interface (`ParseError.Error`, `ValidationError.Error`), les stubs (`StubModule`, utilisés par `registry_test`/`factory_test`), et `third_party/`.
  - **Supprimé** : toute l'API `metadata.Accessor` (17 méthodes + `deepCopyMap`/`deepCopySlice`) ; `logger.FormatMetricsHuman` ; `config.GetEmbeddedSchema` ; `filter.ParseRemoveConfig`/`ParseScriptConfig`/`ParseSetConfig` ; `errhandling.ResolveRetryConfig`/`ResolveErrorHandlingConfig`/`DefaultErrorHandlingConfig`/`RetryExecutor.LastErrors` ; `soapclient.BuildMTOMRequest` — avec leurs tests dédiés.
  - **Conservé (à raison)** : les méthodes du `Scheduler` (`New`, `Unregister`, `HasPipeline`, `GetQueueLength`…) — ce sont l'API d'**observation** que la suite de tests utilise pour vérifier Register/Start/queue (code vivant) ; les constructeurs d'erreur et configs de retry (`NewServerError`, `DefaultRetryConfig`, `NewValidationError`…) — helpers de test ; `registry.ClearRegistries`, `persistence.ParseStatePersistenceConfig`, etc.
  - **Résultat mesuré** : `deadcode -test ./...` = **0** (plus aucune fonction inatteignable depuis main ET les tests) ; `go test ./...` = 2084 passants ; `golangci-lint` = 0 issue.
  - **Note d'exécution** : une limite de dépense mensuelle de l'org a interrompu les sous-agents de suppression en cours de route, laissant `scheduler` temporairement cassé (`New`/`Unregister` retirés à tort) — reconstruits fidèlement et le nettoyage terminé manuellement.
- **Balayage du reste du repo** : `recordpath.ParsePart` (parse `items[0]`) utilise déjà `strings.Index`/`strconv.Atoi` — propre, laissé tel quel. Le tokenizer `internal/template/rewrite.go` reste **volontairement écrit à la main** : la fermeture d'une balise doit ignorer un `}}` à l'intérieur d'une chaîne (`{{ "a}}b" }}`), ce qu'un regexp RE2 ne peut pas exprimer, et c'est une frontière de sécurité qui doit refléter exactement le lexer de gonja.
