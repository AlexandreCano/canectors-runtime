# Known gaps surfaced by the E2E suite

**Aucun gap ouvert.** Les 307 scénarios de `scenarios/` passent (`python3 test-lab/run.py`)
et `scenarios-pending/` est vide.

Ce fichier sert de journal des écarts trouvés par la couverture E2E, et de convention :
un scénario décrivant un comportement non implémenté vit dans `scenarios-pending/`
(non scanné par `run.py`, donc CI verte) jusqu'à ce que l'écart soit tranché.

## Historique des écarts trouvés puis corrigés

### Pagination httpPolling

- **BUG #1 — curseur numérique** (`next_cursor: 1002`) stoppait la pagination après la page 1
  (perte de données silencieuse : `status: success`, aucun log d'erreur). Corrigé :
  `extractStringField` coerce les nombres JSON en string. Les booléens restent
  volontairement non coercés — un `nextCursorField` pointant sur `has_more: false`
  donnerait `"false"`, non vide, et ferait tourner la boucle jusqu'au plafond de pages.
  Scénario : `pagination-cursor-numeric`.
- **BUG #2 — chemins imbriqués** (`meta.next_cursor`, `meta.total_pages`) non résolus alors que
  la doc les montre. Corrigé : résolveur partagé `resolvePath` (dot-notation) appliqué à
  `dataField` / `nextCursorField` / `totalPagesField` / `totalField`.
  Scénario : `pagination-cursor-nested` + `internal/modules/input/http_polling_extract_test.go`.
- **BUG #3 — `limitParam`/`limit` ignorés en `cursor` et `page`** (seul `offset` les émettait).
  Corrigé : helper `paginationLimitParams` + routage via `buildPaginatedURLMultiFrom`.
  Scénario : `pagination-cursor-limit-param`.

### Configuration

- **GAP #4 — la substitution `${VAR}` n'était pas implémentée.** Le placeholder littéral partait
  sur le réseau (vérifié : `Authorization: Bearer ${LAB_BEARER_TOKEN}`), alors que le schéma
  documente `${VAR}` pour tous les credentials et que `concepts/env-vars.mdx` y consacre une page.
  Corrigé : `internal/config/envsubst.go` (`SubstituteEnvVars`), appelé dans le chemin `run`
  après la validation de schéma et avant `ConvertToPipeline`. Substitue les valeurs **string**
  à tout niveau (maps et slices) ; laisse intacts les nombres/booléens, les templates
  `{{record.x}}`, les noms non conformes à `[A-Z_][A-Z0-9_]*`, et `connectionStringRef`
  (résolu par la couche database, dont le schéma exige la forme littérale). Une variable absente
  **ou vide** échoue au démarrage en nommant les variables fautives, sans jamais logger de valeur.
  Scénarios : `auth-input-bearer-env`, `env-subst-endpoint`, `env-subst-missing`
  + `internal/config/envsubst_test.go`.
  *Note* : la substitution s'applique aussi aux champs de métadonnée (`description`…) — écrire
  `${VAR}` en prose dans une description déclenchera donc la résolution.
- **GAP #5 — `http_call` sans `keys` : accepté par `validate`, rejeté à chaque tick du runtime.**
  Le runtime a une règle délibérée (`keyRequired := Body == "" && BodyTemplateFile == ""` dans
  `NewHTTPCallFromConfig`) : sans body, les `keys` sont le seul moyen de paramétrer l'appel par
  record. Le schéma ne l'exprimait pas. Corrigé côté schéma (`filter-schema.json`,
  `$defs/httpCallFilterConfig`) : `if` aucun `body` ni `bodyTemplateFile` `then` `keys` requis
  avec `minItems: 1` — `validate` échoue donc tôt et clairement.
  Test : `TestSchemaHTTPCall_KeysRequiredWithoutBody` dans
  `internal/config/http_call_contract_test.go`.
  *Note* : `soap_call` n'est pas concerné — `soapRequestBase` rend `body` obligatoire.
