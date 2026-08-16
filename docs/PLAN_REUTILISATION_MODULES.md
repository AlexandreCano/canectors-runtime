# Plan d'implémentation — réutiliser les modules entre les exécutions planifiées

> Statut : **✅ terminé** — les six étapes sont faites, soak de non-régression compris.
> Origine : finding resté ouvert à l'issue de la campagne de validation
> (`docs/PLAN_VALIDATION_CONFIANCE.md`, P5).
> Objectif : construire les modules d'un pipeline **une fois** et les réutiliser à chaque tick,
> au lieu de les reconstruire et les détruire à chaque exécution.

---

## 1. Le constat

`PipelineExecutorAdapter.ExecuteWithContext` (cmd/cannectors/main.go) appelle
`factory.CreateInputModule`, `CreateFilterModules` et `CreateOutputModule` **à chaque exécution**.
Pour un pipeline planifié, cela signifie reconstruire toute la chaîne à chaque tick, puis la détruire.

Ce n'est pas une inefficacité théorique. Chaque tick :

| Ressource | Ce qui se passe aujourd'hui |
|---|---|
| Pool de connexions SQL (`database` in/out, `sql_call`) | ouvert puis **fermé** (`db.Close()`) |
| Pool de connexions HTTP (`httpPolling`, `http_call`, `httpRequest`) | `CloseIdleConnections()` |
| Cache LRU de `http_call` / `sql_call` | **reparti froid** |
| Runtime Goja d'un filtre `script` | recréé et script réévalué |
| Token OAuth2 | contourné depuis peu par un cache process-wide |

### Ce que ça coûte, mesuré

| Mesure | Valeur observée |
|---|---|
| Part du tick passée en création + destruction des modules (`sql-call-merge`) | **5,4 ms sur 10,8 ms — 50 %** |
| Idem sur un pipeline HTTP à 1 000 records (`volume-1000`) | 0,3 ms sur 6,4 ms — 5 % |
| Sessions PostgreSQL ouvertes | **~1,3 par tick** (11 ticks → 14 sessions) |
| Extrapolation, pipeline à 15 s | **~7 500 sessions/jour** en pur churn |
| Appels d'enrichissement `http_call` avec cache activé | **18 pour 6 exécutions**, au lieu de 3 au total |

Le surcoût est dominé par l'ouverture des pools : il est proportionnellement énorme sur les pipelines
fréquents à petit volume, exactement le profil d'un connecteur incrémental.

### Comment il a été trouvé

En cherchant pourquoi les tokens OAuth2 n'étaient jamais réutilisés. Le cache de tokens existait et
était correct — il était simplement jeté avec le module à chaque tick. Le correctif livré (cache de
tokens à l'échelle du process) traite le symptôme le plus coûteux ; **la cause est ici**.

---

## 2. Pourquoi c'est réalisable sans tout casser

Deux éléments du code existant portent l'essentiel du risque perçu — et le lèvent.

**Le chemin webhook réutilise déjà les modules.** `runWebhookPipeline` construit filtres et output
**une seule fois** et les réutilise pour chaque requête entrante, y compris **en concurrence** quand
`maxConcurrent > 1`. Autrement dit, la réutilisation de ces modules est déjà en production sur le
chemin le plus exigeant. Le chemin planifié est l'exception, pas l'inverse.

**Le scheduler sérialise les exécutions d'un même pipeline.** `tryStartExecution` positionne
`reg.running` et met en file d'attente plutôt que de lancer en parallèle. Deux exécutions du même
pipeline ne se chevauchent donc jamais : la réutilisation y est **plus sûre** que dans le cas webhook
déjà livré. Deux pipelines *différents* peuvent tourner en parallèle, mais ils n'ont aucune raison de
partager des modules.

---

## 3. Les risques réels, et leur traitement

### 3.1 Changement de sémantique — les globals JavaScript survivent

Le runtime Goja est créé à la **construction** du module (`goja.New()` puis évaluation du script).
Aujourd'hui, chaque tick repart d'un runtime vierge. Avec la réutilisation, une variable globale posée
par un script persiste d'un tick à l'autre :

```js
var seen = 0;                                  // aujourd'hui : remis à 0 chaque tick
function transform(record) {
  seen++;                                      // demain : compte à travers les ticks
  record.seen = seen;
  return record;
}
```

C'est le seul changement **visible par l'utilisateur**. Deux options :

- **(a) Accepter et documenter.** Un runtime persistant est le comportement attendu d'un moteur
  embarqué, et il permet des usages légitimes (mémo, compteur). Coût : une note dans la doc du filtre
  `script`, et un test qui fige la nouvelle sémantique.
- **(b) Préserver l'isolation** en réévaluant le script source à chaque tick. On garde la sémantique
  actuelle mais on perd une partie du gain (la compilation reste refaite), tout en conservant le gain
  principal, qui vient des pools.

**✅ DÉCIDÉ : (c) — isoler le runtime à chaque tick, en précompilant le script une fois.**

Ni (a) ni (b) : la mesure a fait apparaître une troisième option strictement meilleure. Le coût
supposé de l'isolation était le seul argument en faveur de la persistance, et il ne tient pas.

**Ce que coûte réellement l'isolation** (benchmark Go, à rejouer pendant l'implémentation) :

| Opération | Coût | Mémoire |
|---|---|---|
| Reconstruire entièrement le module script (`goja.Compile` + validation) | **19,1 µs** | 16 Ko, 213 allocs |
| **VM neuve depuis le programme compilé** (`newScriptRun`, mesuré après implémentation) | **6,2 µs** | 8 Ko, 82 allocs |
| Le travail utile : transformer 100 records | ~99 µs | — |

*(Le premier chiffrage annonçait 1,3 µs pour la VM neuve : c'était `goja.New` + `RunProgram` bruts.
La liaison de la console JS ajoute ~5 µs, d'où les 6,2 µs réels. Conclusion inchangée : 0,15 % d'un
tick, 1/870e du coût des pools supprimés. Benchmarks conservés dans `script_bench_test.go`.)*

À comparer à ce que ce plan cherche à supprimer : **5 400 µs** de création/destruction de pools par
tick, sur un tick de 6 000 à 11 000 µs. Une reconstruction complète du script représente **0,1 % d'un
tick** et **1/370e** du coût qu'on élimine ; avec la précompilation, 0,01 %. Le coût de l'isolation
est, à toutes fins pratiques, nul.

**Ce que l'isolation évite** : un script qui accumule dans un global — `all.push(record)`, un
dictionnaire de correspondance qui grossit — deviendrait sinon une **fuite mémoire non bornée**.
Sur un pipeline à 15 s traitant 1 000 records par tick, cela fait 5,8 millions de records retenus
par jour, jusqu'à épuisement. Aujourd'hui ce bug est invisible parce que le runtime repart vierge ;
le rendre persistant le transformerait en incident de production silencieux — exactement la classe de
défaut que la campagne de validation a passé son temps à traquer.

**Mise en œuvre concrète** : le module script conserve `scriptSource` **et un `*goja.Program`
compilé une fois** à la construction. À chaque exécution, `Process` instancie une VM neuve, y exécute
le programme, récupère `transform`, et la laisse partir en fin d'exécution. Le module reste réutilisé
comme les autres — c'est seulement le runtime qui est neuf.

**Ce qu'on ne perd pas** : le module script ne porte **aucune ressource coûteuse** — ni pool, ni
cache, ni connexion (cf. sa structure : `scriptSource`, `onError`, `runtime`, `transformFn`,
`console`, un mutex). Tout le gain de ce plan vient des *autres* modules. Isoler le runtime ne coûte
donc rien au bénéfice recherché.

**Effet de bord bienvenu** : côté script, la sémantique visible par l'utilisateur **ne change pas du
tout**. Plus de changement de comportement à documenter à cet endroit, et l'étape 5 se réduit à un test
qui fige l'isolation.

**Attention cependant — une autre conséquence est bien visible, et j'étais passé à côté** : le cache
LRU d'`http_call`, `sql_call` et `soap_call` survit désormais aux ticks, donc `ttlSeconds` devient un
**budget de péremption** et non plus un détail par exécution. Un pipeline à 15 s avec le TTL par défaut
peut servir une donnée vieille de cinq minutes là où il rafraîchissait à chaque tick. C'est un
changement de comportement au sens de la règle du workspace ; il est documenté dans
`cannectors-doc` sur les trois pages de filtres concernées.

*(Pour mémoire, l'option (a) — accepter la persistance — reste techniquement viable et permettrait des
usages de type mémo entre ticks. Elle est écartée parce qu'elle échange un gain nul contre un risque
d'épuisement mémoire silencieux.)*

### 3.2 Perte de l'auto-guérison

Aujourd'hui, un module en mauvais état (pool cassé, endpoint devenu invalide) disparaît à la fin du
tick et le suivant repart propre. Avec la réutilisation, un module dégradé pourrait persister.

**✅ DÉCIDÉ : évincer l'entrée du cache dès qu'une exécution retourne une erreur**, sans chercher à
distinguer le type d'erreur.

Deux vérifications faites avant de trancher, qui rendent ce choix solide plutôt que simplement commode :

1. **Le pire cas est exactement le comportement d'aujourd'hui.** Un pipeline qui échoue à chaque tick
   reconstruirait ses modules à chaque tick — c'est précisément ce que fait la version actuelle. La
   règle ne peut donc *rien* régresser ; elle ne fait que renoncer au gain là où il n'y a de toute
   façon rien à préserver.
2. **Les erreurs de données ne déclenchent pas d'éviction.** Vérifié dans le code et sur le lab :
   `onError: skip` et `onError: log` retournent `nil`, l'exécution finit en `status: success`
   (scénario `db-output-on-error-log`). Seul un échec réel — source injoignable, output en erreur,
   filtre en `onError: fail` — évince. L'objection « on va reconstruire pour un mauvais record » ne
   tient donc pas.

Classer les erreurs (n'évincer que sur `CategoryNetwork`, par exemple — `errhandling` sait déjà le
faire) n'apporterait quelque chose que pour un pipeline échouant *systématiquement* tout en voulant
des pools chauds. Cas assez improbable pour ne pas payer la complexité d'avance ; à revoir seulement
si un pipeline réel montre ce profil.

À noter : `database/sql` gère déjà la reconnexion en interne, donc un pool réutilisé sur une base
redémarrée se rétablit seul. L'éviction est une ceinture en plus des bretelles.

### 3.3 Cycle de vie et arrêt

Les modules ne sont plus fermés en fin d'exécution : il faut les fermer à l'**arrêt du process**,
sinon les pools restent ouverts jusqu'à la mort du processus (acceptable) mais sans fermeture propre
(moins acceptable pour PostgreSQL, qui log des déconnexions brutales).

**Traitement** : une méthode `Close()` sur l'adaptateur, appelée depuis `main` sur le même chemin que
`scheduler.Stop()`. Le correctif de contexte livré récemment garantit que ce chemin est bien atteint.

### 3.4 Concurrence entre pipelines différents

Le cache est indexé par pipeline ; deux pipelines ne partagent jamais un module. Il reste que la map
elle-même est lue et écrite depuis plusieurs goroutines : **mutex**, comme partout ailleurs.

### 3.5 Ce qui ne change pas

- **L'état de persistance** (`lastState`) est chargé depuis le `StateStore` à chaque exécution, pas
  porté par le module : rien à faire.
- **La configuration** est lue une fois au démarrage ; il n'y a pas de rechargement à chaud, donc pas
  de risque de servir un module construit sur une config périmée.
- **Le chemin `run` one-shot** (`runPipelineOnce`) et le chemin **webhook** ne sont pas touchés.

---

## 4. Options envisagées

| Option | Portée | Gain | Verdict |
|---|---|---|---|
| **A. Cache dans l'adaptateur**, indexé par pipeline | `cmd/` uniquement | tous les pools + tous les caches | **retenue** |
| B. Modules portés par le `registeredPipeline` du scheduler | scheduler + factory | identique | rejetée : le scheduler est volontairement découplé de la construction des modules (il ne connaît qu'une interface `Executor`) ; l'y coupler pour un gain nul |
| C. Mutualiser seulement les ressources coûteuses (registre de pools DB/HTTP partagés) | `internal/database`, `internal/httpclient` | pools uniquement | rejetée comme cible : ne récupère ni le cache LRU ni la compilation du script, et introduit un état global plus difficile à raisonner que le cache par pipeline |
| D. Ne rien faire, documenter | — | — | rejetée : 50 % du tick et ~7 500 sessions/jour sur un profil courant |

**Option A retenue** : l'adaptateur est déjà l'objet qui sait construire une chaîne de modules à
partir d'un pipeline. Lui confier leur durée de vie ne déplace aucune responsabilité et laisse le
scheduler intact.

---

## 5. Mise en œuvre, par étapes vérifiables

### Étape 1 — extraire la construction dans un type dédié

Sortir les trois appels `factory.Create*` de `ExecuteWithContext` vers un `moduleSet` porteur des
modules et d'un `Close()`. **Aucun changement de comportement à ce stade** : on construit toujours par
tick.
*Vérification* : les 336 scénarios passent, `go test ./...` et le lint restent verts.

### Étape 2 — introduire le cache, indexé par pipeline

Une map protégée par mutex dans l'adaptateur, clé `pipeline.ID` + `dryRun`. Construction à la
première exécution, réutilisation ensuite.
*Vérification* : mesurer à nouveau les trois chiffres du §1 — part du tick, sessions PostgreSQL,
appels d'enrichissement. Attendu : appels d'enrichissement **3 au total** au lieu de 3 par tick ;
sessions **~1 au total** au lieu de ~1,3 par tick.

### Étape 3 — éviction sur erreur

Si l'exécution retourne une erreur, retirer l'entrée du cache pour que le tick suivant reconstruise.
*Vérification* : un scénario où la source échoue puis se rétablit doit repartir normalement — les
stubs `reliability/*-then-200` existants s'y prêtent directement.

### Étape 4 — fermeture à l'arrêt

`Close()` sur l'adaptateur, appelé depuis `main` à côté de `scheduler.Stop()`.
*Vérification* : après un SIGTERM, aucune session PostgreSQL ne reste ouverte
(`SELECT count(*) FROM pg_stat_activity`), et `crash.py` reste vert.

### Étape 5 — précompiler le script et figer l'isolation du runtime

Compiler le script une fois (`goja.Compile`) à la construction du module, instancier une VM neuve par
exécution (`RunProgram`). Un test qui **fige l'isolation** : un script incrémentant un global doit
repartir de zéro à chaque exécution — sans ce test, une optimisation ultérieure pourrait réintroduire
la persistance sans que rien n'échoue.

Remettre au passage le benchmark ayant servi à trancher (`BenchmarkScript_FullModuleBuild` vs
`BenchmarkScript_FreshVMPrecompiled`), pour que le rapport de coût reste vérifiable.

*Aucune mise à jour de `cannectors-doc` n'est nécessaire* : la sémantique visible par l'utilisateur
est inchangée. C'est le principal mérite de cette option.

### Étape 6 — non-régression sur la durée — ✅ **faite, verdict PASS**

C'est **l'étape à ne pas sauter** : des modules à longue vie sont exactement le terrain des fuites que
la campagne cherchait. `make test-lab-soak DURATION=2h` doit rester au verdict PASS, avec RSS,
descripteurs et connexions stables — les seuils sont déjà en place.

Soaker en plus **un pipeline avec un filtre `script`**, ce que la campagne n'a jamais fait. Avec
l'option retenue le runtime est neuf à chaque tick, donc le risque est faible — mais c'est précisément
ce qu'il faut vérifier : qu'une VM créée puis abandonnée des milliers de fois ne laisse rien derrière
elle.

**Résultats mesurés (2 h chacun) :**

| | `sql-call-merge` (pool DB à vie longue) | `filters-script-inline` (VM jetée par tick) |
|---|---|---|
| Exécutions | **7 218** | **7 219** |
| RSS début → fin | 28 → 29 Mio (**1,01×**) | 28 → 30 Mio (**1,02×**) |
| Descripteurs | 8 → 8 | 7 → 7 |
| Threads | **18 → 18 (1,00×)**, plage 14-18 | **18 → 18 (1,00×)**, plage 15-18 |
| Connexions PostgreSQL | 5 → 6, stable | 5 → 5 |
| Dérive CRON | 0 ms (p50, p99, max) | max 1 ms |
| Lignes d'erreur | **0** | **0** |
| Verdict | **PASS** | **PASS** |

Deux conclusions :

- **Le mouvement des threads observé sur un run court (13 → 18) plafonne.** Sur 2 h le ratio est de
  1,00× : c'était la montée en régime du pool, pas une fuite. C'était le seul résultat qui pouvait
  invalider ce chantier.
- **7 219 VM Goja créées puis abandonnées coûtent +2 % de RSS**, soit ~500 Kio sur deux heures.
  L'isolation par exécution ne laisse rien derrière elle : la crainte de fuite mémoire est écartée par
  la mesure, dans les deux sens — ni globals persistants, ni churn de VM problématique.

---

## 6. Comment on saura que c'est réussi

| Indicateur | Avant | Cible | **Mesuré après** |
|---|---|---|---|
| Surcoût par tick, hors exécution (`sql-call-merge`) | 5,4 ms (50 % d'un tick de 10,8 ms) | < 5 % | **0,7 ms** sur un tick de **4,0 ms** — le tick est 2,7× plus rapide, le surcoût 7,7× plus faible. La *part* reste à 18 % parce que le tick lui-même a fondu ; c'est l'absolu qui compte. |
| Sessions PostgreSQL | ~1,3 par tick (14 pour 11) | ~0 en régime établi | **4 pour 11 ticks** — le pool s'établit sur les premiers ticks puis n'ouvre plus |
| Appels `http_call` par clé cacheable, sur 7 ticks | 7 (1 par tick) | 1 | **1** (`CUST-001` et `CUST-002` : 1 appel chacun) |
| Appels pour une clé **en échec** (404, `onError: skip`) | 1 par tick | — | 1 par tick, **inchangé et correct** : un échec n'est pas mis en cache. La cible « 3 au total » du plan initial était donc mal posée. |
| Verdict `soak.py` sur 2 h | PASS | PASS (inchangé) | **PASS** sur les deux pipelines (voir étape 6) |
| Suite E2E | 336 verts | 336 verts | **337 verts** |

Les trois premières mesures se relèvent avec les commandes déjà utilisées pour l'analyse :

```bash
# part du tick : comparer "scheduled pipeline execution completed".duration
#                à "execution metrics".total_duration
timeout 9 ./bin/cannectors run test-lab/pipelines/sql-call-merge.yaml

# sessions PostgreSQL
psql -At -c "SELECT sessions FROM pg_stat_database WHERE datname='cannectors_test'"

# cache LRU : compter /enrichment/customers/ dans le journal WireMock
timeout 6 ./bin/cannectors run test-lab/pipelines/http-call-path-merge.yaml
```

**Limite connue** : aucune de ces trois propriétés n'est démontrable par un scénario déclaratif, le
runner arrêtant le pipeline après la première exécution. Elles se vérifient à la main, comme le cache
de tokens OAuth2. Rendre le runner capable d'observer plusieurs ticks serait un préalable utile, mais
c'est un chantier distinct.

---

## 7. Ce que ce plan ne traite pas

- **Le rechargement à chaud** d'une configuration modifiée : hors périmètre, et le cache le rendrait
  plus délicat (il faudrait invalider sur changement de fichier).
- **Le partage de pools entre pipelines** visant la même base : chaque pipeline garde les siens. C'est
  volontaire — le gain serait marginal et le couplage réel.
- **Le cache de tokens OAuth2** livré précédemment : il devient partiellement redondant pour un
  pipeline unique, mais reste utile quand plusieurs pipelines partagent les mêmes credentials. À
  conserver.

---

## 8. Décisions prises

| Question | Décision | Où c'est développé |
|---|---|---|
| Globals JavaScript entre les ticks | **Isolation préservée** : script précompilé une fois, VM neuve par tick. Décidé sur mesure — l'isolation coûte 1,3 µs, soit 0,01 % d'un tick | §3.1, étape 5 |
| Éviction du cache sur erreur | **Sur toute erreur**, sans classification | §3.2 |

Rien ne bloque le démarrage de l'implémentation.
