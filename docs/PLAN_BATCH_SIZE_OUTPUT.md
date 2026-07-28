# Plan d'implémentation — `batchSize` : découpage des requêtes de sortie en lots

> Statut : **📋 À IMPLÉMENTER**
> Objectif : permettre à un output en `requestMode: batch` d'émettre **plusieurs requêtes de N records** au lieu d'une seule requête contenant tous les records, via un champ de configuration `batchSize`.

---

## 1. Contexte et besoin

Un pipeline Smartsheet → Praxedo lit les lignes d'un sheet et les envoie à l'opération SOAP `createEvents` (pluriel : l'enveloppe accepte plusieurs `<events>`). Depuis l'unification du templating sur Jinja2, le body peut boucler sur le lot :

```jinja
{%- for row in record.records %}
  <events>…</events>
{%- endfor %}
```

Le besoin exprimé : **borner le nombre de records par requête** (« max 50 par batch »). Aujourd'hui c'est binaire — soit une requête pour tout le sheet, soit une requête par ligne.

Motivations d'un découpage :

- **Limites du service distant** — taille de payload, nombre d'entités par appel, timeout serveur.
- **Rayon d'explosion** — en `batch`, un rejet fait échouer la totalité ; en lots de 50 l'échec est circonscrit.
- **Mémoire / latence** — une enveloppe XML de plusieurs milliers d'événements est coûteuse à sérialiser côté client comme à parser côté serveur.

---

## 2. État des lieux (audit du code existant)

### 2.1 Aucun mécanisme de découpage n'existe

`grep -rn "batchSize\|chunk"` sur `internal/`, `cmd/`, `pkg/` → **0 résultat**. Le concept est absent du code comme des schémas.

### 2.2 Le chemin d'exécution est un appel unique

| Étape | Localisation | Comportement |
|---|---|---|
| Executor | `internal/runtime/pipeline.go:270` | `e.outputModule.Send(ctx, records)` — **un seul appel**, avec la totalité des records post-filtres |
| Pagination input | `internal/modules/input/http_polling_pagination.go:67` | agrège toutes les pages dans `allRecords` avant de rendre la main → aucun découpage naturel en amont |
| soapRequest batch | `internal/modules/output/soap_request.go:117` `sendBatch` | `batchRecord(records)` puis **un** `sendRecord` |
| httpRequest batch | `internal/modules/output/http_request.go:376` `sendBatchMode` | **une** requête, body = tableau JSON de tous les records |

Le découpage ne peut donc se faire **que dans l'output**, à l'intérieur de `Send`.

### 2.3 Le mode `single` fournit le modèle de boucle et de gestion d'erreur

`soap_request.go:125` `sendSingle` est la référence à suivre : boucle avec vérification de `ctx.Done()`, compteur `sent` incrémental, et branchement sur `onError` :

- `OnErrorFail` → retour immédiat avec `sent` partiel et erreur contextualisée
- `OnErrorSkip` / `OnErrorLog` → log de l'échec et poursuite

`http_request.go:481` `sendSingleRecordMode` suit le même schéma.

### 2.4 Ce que voit le template en mode batch

`soap_request.go:302` :

```go
func batchRecord(records []map[string]any) map[string]any {
	items := make([]any, 0, len(records))
	for _, record := range records {
		items = append(items, record)
	}
	return map[string]any{"records": items, "recordCount": len(records)}
}
```

Le template voit `record.records` et `record.recordCount`. **Conséquence directe pour le découpage** : en appelant `batchRecord` par lot, `record.records` contient les records du lot et `recordCount` sa taille — le template n'a **rien à changer**. C'est le point qui rend cette feature peu coûteuse.

### 2.5 Divergences existantes du mode batch de httpRequest (dette à connaître)

Deux comportements de `sendBatchMode` sont surprenants et vont interagir avec le découpage :

1. **`bodyTemplateFile` en batch n'utilise que `records[0]`** (`http_request.go:~380`, commentaire explicite : *« For batch mode with template, we can only use first record's data »*). Contrairement à soapRequest, httpRequest ne construit **pas** de `batchRecord` — un template de body HTTP ne peut donc pas boucler sur le lot. Le body par défaut (marshal JSON du tableau de records) est le seul chemin réellement « batch ».
2. **Endpoint et headers résolus depuis `records[0]`** (`http_request.go:~440-455`, `resolveEndpointForBatch` + `extractHeadersFromRecord(records[0])`).

Ce plan **ne corrige pas** ces divergences (hors périmètre), mais elles sont à documenter car avec `batchSize` elles s'appliqueront **par lot** (chaque lot résout son propre endpoint/headers depuis son premier record), ce qui est un changement observable.

### 2.6 Le dry-run doit suivre

`soap_request.go:222` `PreviewRequest` et `http_request_preview.go:29` produisent **une** preview en mode batch. Si `Send` émet N requêtes et que `PreviewRequest` en montre une seule, le `--dry-run` mentirait sur ce qui sera réellement envoyé. Le découpage doit être appliqué **aux deux chemins**, depuis la même fonction de découpage.

### 2.7 Output `database` — hors périmètre

`database.go:133` `Send` exécute **une query par record** (`sendWithTransaction` / `sendWithoutTransaction`) ; il n'y a pas de notion de requête agrégée, donc pas de lot à dimensionner. Le champ `transaction` couvre déjà le regroupement. `batchSize` n'a pas de sens ici et ne sera pas ajouté.

---

## 3. Décisions de conception

| Sujet | Décision | Justification |
|---|---|---|
| **Nom du champ** | `batchSize` | Aligné sur `requestMode`, sans préfixe redondant. `maxBatchSize` écarté : la taille est exacte pour tous les lots sauf le dernier, « max » suggère une borne molle. |
| **Portée** | `soapRequest` **et** `httpRequest` | Deux outputs avec un `requestMode: batch` identique ; n'en équiper qu'un créerait deux sémantiques batch divergentes. Phasé : soapRequest d'abord (le besoin réel), httpRequest ensuite. |
| **Valeur par défaut** | absent = un seul lot | Rétrocompatibilité de comportement pour les pipelines existants ; pas de valeur par défaut arbitraire. |
| **`batchSize` + `requestMode: single`** | **erreur au build du module** | La combinaison n'a pas de sens. Un warning silencieux laisserait croire au découpage. Cohérent avec la maturité du projet (erreurs franches, pas de tolérance). |
| **Découpage** | `slices.Chunk` (stdlib, Go 1.23+) | Règle « check library first » : `go 1.25.0` dans `go.mod`, aucune raison d'écrire une boucle d'indices maison. |
| **Sémantique d'erreur** | Calquée sur `sendSingle` : `fail` → arrêt avec compte partiel ; `skip`/`log` → poursuite des lots suivants | Un utilisateur qui connaît `single` n'a pas de nouveau modèle mental à acquérir. |
| **`recordCount` dans le template** | taille **du lot**, pas du total | Conséquence mécanique de `batchRecord` par lot. À documenter explicitement : un template qui affichait « X événements au total » changera de sens. |
| **Dry-run** | `PreviewRequest` produit une preview **par lot** | Sans ça le dry-run ne représente plus la réalité (cf. §2.6). |

### Point ouvert

**Ordre des lots et parallélisme** : ce plan émet les lots **séquentiellement**. Un envoi concurrent (N lots en parallèle) serait un gain de latence mais soulève la limitation de débit côté serveur, l'ordre d'application des événements, et la sémantique de `onError`. **Hors périmètre** — à traiter séparément si le besoin apparaît.

---

## 4. Architecture cible

```
Executor ──Send(ctx, records)──▶ Output module
                                    │
                                    ├─ requestMode == "single" ──▶ 1 requête / record   (inchangé)
                                    │
                                    └─ requestMode == "batch"
                                         │
                                         ├─ batchSize == 0 ──▶ 1 requête / tous records (inchangé)
                                         │
                                         └─ batchSize == N ──▶ slices.Chunk(records, N)
                                                                 │
                                                                 └─ pour chaque lot :
                                                                      batchRecord(lot) ──▶ 1 requête
```

### 4.1 Helper partagé

Les deux outputs vivent dans le même package `internal/modules/output`. Le découpage et la validation sont donc factorisés là, à côté de `normalizeRequestMode` (`http_request_success.go:23`) :

```go
// normalizeBatchSize valide batchSize au regard du requestMode.
// Retourne 0 lorsque aucun découpage ne doit être appliqué.
func normalizeBatchSize(batchSize int, requestMode, moduleType string) (int, error)
```

- `batchSize == 0` (absent) → `0, nil`
- `batchSize < 0` → erreur (filet ; le schéma le rejette déjà)
- `batchSize > 0` et `requestMode == "single"` → erreur explicite
- sinon → `batchSize, nil`

### 4.2 Fonction de découpage unique

Une seule source de vérité, consommée par `Send` **et** par `PreviewRequest` des deux modules :

```go
// chunkRecords découpe records en lots de size. size <= 0 renvoie un lot unique.
func chunkRecords(records []map[string]any, size int) [][]map[string]any
```

Implémentée sur `slices.Chunk` ; le cas `size <= 0` renvoie `[][]map[string]any{records}` pour que les appelants n'aient **aucun branchement** à faire — le chemin « pas de découpage » est le chemin découpé avec un seul lot. C'est ce qui garantit que le comportement historique et le nouveau ne divergent pas.

Deux points de l'API stdlib à respecter (`$GOROOT/src/slices/iter.go:97`) :

- **`slices.Chunk` panique si `n < 1`** — le garde `size <= 0` de `chunkRecords` n'est donc pas cosmétique, il est obligatoire avant l'appel.
- La signature est `func Chunk[Slice ~[]E, E any](s Slice, n int) iter.Seq[Slice]` : elle rend un **itérateur**, pas une slice de slices. `chunkRecords` fait donc `slices.Collect(slices.Chunk(records, size))` — la matérialisation est nécessaire puisque `PreviewRequest` doit pré-dimensionner sa slice de previews et que les tests comptent les lots.

---

## 5. Plan d'implémentation par étapes

> Chaque phase se termine par `go test ./...` + `golangci-lint run ./...` au vert (règle projet).

### Phase 1 — Helpers partagés + schéma

1. `internal/modules/output/` : ajouter `chunkRecords` et `normalizeBatchSize` (fichier `batching.go`), avec leurs tests unitaires — découpage exact, reste partiel, `size >= len(records)`, `size <= 0`, slice vide.
2. `internal/config/schema/output-schema.json` : ajouter `batchSize` à `soapRequestOutputConfig` et `httpRequestOutputConfig` :

```json
"batchSize": {
  "type": "integer",
  "minimum": 1,
  "description": "Maximum number of records per request when requestMode is 'batch'. When omitted, all records are sent in a single request. Invalid with requestMode 'single'."
}
```

3. Contract tests `internal/config/soap_contract_test.go` et `http_request_contract_test.go` : `batchSize: 50` accepté, `batchSize: 0` et `batchSize: -1` rejetés.

→ **verify** : `go test ./internal/modules/output/... ./internal/config/...` + lint.

### Phase 2 — `soapRequest`

1. `SOAPRequestOutputConfig` : champ `BatchSize int \`json:"batchSize,omitempty"\``.
2. `SOAPRequestModule` : champ `batchSize int`, renseigné dans `NewSOAPRequestFromConfig` via `normalizeBatchSize(cfg.BatchSize, requestMode, "soapRequest")` — juste après `normalizeRequestMode` (`soap_request.go:59`).
3. `sendBatch` (`soap_request.go:117`) réécrit sur `chunkRecords` : boucle sur les lots, `ctx.Done()` en tête de boucle (comme `sendSingle`), `batchRecord(lot)` → `sendRecord`, compteur `sent` cumulé, branchement `onError` identique à `sendSingle`.
4. `PreviewRequest` (`soap_request.go:222`) : en mode batch, une preview par lot issu du **même** `chunkRecords`.
5. Log de création du module : ajouter `slog.Int("batch_size", batchSize)` à côté de `request_mode` (`soap_request.go:99`). Logs d'envoi : `batch_index` / `batch_count` pour rendre le découpage observable.

**Tests** (`soap_request_test.go`) :

- 120 records, `batchSize: 50` → 3 requêtes reçues, de tailles 50/50/20 ; total `sent` = 120.
- `recordCount` du template vaut la taille du lot (assertion sur le body reçu).
- Échec du 2ᵉ lot avec `onError: fail` → `sent` = 50 et erreur remontée ; avec `onError: skip` → 3 requêtes tentées, `sent` = 70.
- Annulation de contexte entre deux lots → arrêt propre, compte partiel.
- `batchSize` absent → **une seule** requête (non-régression).
- `batchSize` + `requestMode: single` → erreur au build du module.
- `PreviewRequest` avec `batchSize: 50` sur 120 records → 3 previews.

→ **verify** : `go test ./internal/modules/output/...` + lint + un pipeline SOAP réel validé au binaire.

### Phase 3 — `httpRequest`

Même structure : champ de config, `normalizeBatchSize`, `sendBatchMode` (`http_request.go:376`) sur `chunkRecords`, `previewBatchMode` (`http_request_preview.go:39`) aligné.

Spécificités à traiter explicitement :

- Le body par défaut (marshal JSON) est calculé **par lot** → un tableau JSON par requête.
- `resolveEndpointForBatch` et `extractHeadersFromRecord(records[0])` s'appliquent **par lot** (cf. §2.5) — ajouter un test qui fige ce comportement avec un endpoint templatisé, pour qu'il soit choisi et non subi.
- La limite `bodyTemplateFile` = `records[0]` (§2.5) est **conservée** ; ajouter un commentaire renvoyant à cette dette plutôt que de la corriger ici.

→ **verify** : `go test ./...` (les deux outputs partagent des helpers → périmètre large) + lint.

### Phase 4 — Documentation (`cannectors-doc/`)

Règle du workspace : même session de développement.

1. `pnpm sync-schemas ../cannectors` puis `pnpm check-schemas`.
2. `content/docs/modules/outputs/soap-request/index.mdx` et `http-request/index.mdx` : section **Batching** — `requestMode` × `batchSize`, tableau de comportement, exemple à 50, et les trois points de vigilance : `recordCount` = taille du lot, échec partiel possible, retry rejoué par lot.
3. `content/docs/concepts/retry-error-handling.mdx` : préciser l'interaction `onError` × lots (`fail` arrête au lot fautif, les lots déjà envoyés le restent).
4. Si l'exemple Smartsheet/SOAP de `examples/` est mis à jour, refléter dans `content/docs/examples/`.

→ **verify** : `pnpm lint`, `pnpm types:check`, `pnpm check-links`, `pnpm build`.

---

## 6. Risques et points de vigilance

| # | Risque | Traitement |
|---|---|---|
| R1 | **Perte d'atomicité.** En `batch` sans `batchSize`, l'envoi est tout-ou-rien. Avec des lots, un échec au 3ᵉ lot laisse les 2 premiers appliqués côté serveur. | Comportement inhérent au besoin, pas un défaut. Documenté, et `onError: fail` + le compte partiel remonté par `Send` permettent de le constater. Aucune compensation (rollback) n'est envisagée. |
| R2 | **Retry non idempotent.** Le retry existant s'applique par requête, donc par lot : un lot en timeout mais réellement appliqué serait rejoué et pourrait créer des doublons. | Pré-existant en mode `single`, amplifié ici. À documenter : l'idempotence est la responsabilité du service distant (clé métier `<id>` côté Praxedo). |
| R3 | **`recordCount` change de sens** pour un template existant qui passerait à `batchSize`. | Documenté en §3 et dans la doc. Aucun pipeline actuel n'utilise `batchSize` (nouveau champ), donc pas de régression silencieuse possible. |
| R4 | **Divergence Send / Preview.** Deux implémentations du découpage dériveraient et le dry-run mentirait. | `chunkRecords` est l'unique source de vérité, appelée par les deux chemins (§4.2). Test dédié : nombre de previews == nombre de requêtes émises pour le même jeu de records. |
| R5 | **Métriques.** `recordsSent` / `recordsFailed` sont calculés par l'executor comme `len(records) - sent` (`pipeline.go:~275`). | Reste correct : `sent` est le cumul des records des lots réussis. Vérifier par un test que le compte est exact sur un échec de lot médian. |

---

## 7. Critères de succès

- `batchSize: 50` sur 120 records produit **3 requêtes** de 50/50/20 sur `soapRequest` comme sur `httpRequest`.
- `batchSize` absent → comportement **strictement identique** à aujourd'hui (une requête), non-régression couverte par test.
- `batchSize` avec `requestMode: single` → erreur au build du module, et `batchSize: 0` → erreur de schéma dès `cannectors validate`.
- `--dry-run` affiche autant de previews que de requêtes réellement émises.
- Un seul point de découpage dans le code (`chunkRecords`), sur `slices.Chunk`.
- `go test ./...` + `golangci-lint run ./...` au vert ; exemples revalidés au binaire ; `cannectors-doc/` à jour et `pnpm build` réussi.
