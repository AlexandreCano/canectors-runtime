# Plan de validation — passer du « vert en laboratoire » au « digne de confiance en production »

> Statut : **P0 et P2 à P6 terminés**, harnais **P1 prêt** (run longue durée à lancer).
> Reste P7 (pilote réel), qui est une activité opérationnelle et non une branche.
> Point de départ : 307 scénarios E2E verts, 2118 tests unitaires, race detector en CI, lint propre.
> Objectif : combler les angles morts que la suite actuelle ne peut **structurellement** pas couvrir,
> et remplacer une confiance déclarative par des preuves mesurables.

---

## 1. Où on en est, honnêtement

Ce qui est acquis :

| Acquis | Preuve |
|---|---|
| Chaque option de chaque module est exercée au moins une fois | 307 scénarios déclaratifs (`test-lab/run.py`) |
| Les croisements à risque sont couverts | 212 cellules générées (`test-lab/generate-matrix.py`), 16 axes |
| Les 3 drivers SQL fonctionnent | scénarios postgres / sqlite / mysql |
| Pas de course de données détectée | `go test -race` en CI (`.github/workflows/ci.yml`) |
| Les valeurs SQL sont liées, jamais interpolées | `internal/sqltemplate` + scénarios `sql-call-*` |

Ce que cette suite **ne peut pas** dire, et pourquoi ça compte :

| Angle mort | Pourquoi c'est risqué |
|---|---|
| Tout tourne contre WireMock en **localhost** | Aucun TLS, DNS, coupure en milieu de corps, proxy, HTTP/2, ni bizarrerie d'API réelle |
| Volumes de **1 à 5 records** | Rien sur la mémoire, la taille des batches, la backpressure, les limites de pagination réelles |
| Durée de vie **~1 seconde** puis SIGTERM | Aucune information sur les fuites (goroutines, connexions), la dérive du CRON, l'expiration d'un token OAuth2 |
| Aucun **crash/reprise** | État partiellement écrit, fichier d'état corrompu, deux runs concurrents du même pipeline |
| **Sémantique de livraison** non testée | Après un retry sur un batch partiellement écrit : duplication ? perte ? Le finding `transaction`×`onError` montre que la zone est mince |
| **Webhook** = surface la moins couverte | C'est la seule exposée à Internet ; encore testée par scripts shell impératifs |
| Deux affirmations **non auditées** | « les secrets ne sont jamais loggés » ; garde-fou sur `{{ record.* }}` dans le *texte* SQL |
| **Angles morts corrélés** | Les assertions ont été dérivées du code par le même agent qui l'a lu — mêmes malentendus possibles |

### Le signal le plus important

5 défauts réels ont été trouvés dès la première campagne, dont **2 pertes de données silencieuses**
(`status: success`, aucun log d'erreur) dans la pagination cursor — la fonctionnalité la plus utilisée —
qui avaient survécu à 24 epics et 2118 tests unitaires. Le curseur numérique est un cas
**très courant** dans les vraies APIs, pas un cas tordu.

Conclusion opérationnelle : **le taux de découverte quand on regarde un endroit neuf est élevé.**
Le plan ci-dessous priorise donc les endroits jamais regardés, pas l'approfondissement de ce qui est déjà vert.

---

## 2. Trois principes directeurs

1. **Des invariants plutôt que des assertions ponctuelles.** La classe « perte silencieuse » se
   détecte structurellement (`N entrés = N sortis`), pas cas par cas. Un invariant global attrape
   les bugs qu'on n'a pas imaginés — c'est exactement ce qui manquait pour le curseur numérique.
2. **Sortir du localhost.** Tant que la seule contrepartie est WireMock en boucle locale, une classe
   entière de défaillances est hors d'atteinte.
3. **Casser la corrélation.** Là où c'est possible, dériver les attendus de la **doc** (ou par
   propriété/fuzzing) et non du code, pour ne pas rejouer les malentendus de l'implémentation.

---

## 3. Phases, par rendement décroissant

### P0 — Invariant de réconciliation `N in = N out` — ✅ **terminé**

**Objectif** : rendre impossible une perte silencieuse non détectée, sur *tous* les scénarios existants
et futurs, sans écrire une assertion par cas.

Le runtime logge déjà ce qu'il faut : `input fetch completed{record_count}`,
`output send completed{records_sent, records_failed}`, `execution metrics{records_processed}`.

**Tâches**
- Ajouter un type d'assertion `reconcile` à `test-lab/run.py` : compare `record_count` (input),
  `records_sent` + `records_failed` (output), et le nombre d'enregistrements réellement reçus par
  la contrepartie (journal WireMock / `COUNT(*)` SQL).
- L'appliquer **par défaut** à tout scénario dont le statut attendu est `success` (opt-out explicite
  pour les cas où un filtre supprime volontairement des records, ex. `drop`, `onError: skip`).
- Étendre le générateur pour émettre l'invariant dans chaque cellule pertinente.
- Ajouter un fixture source à **N records paramétrable** (100, 1 000, 10 000) et vérifier
  l'invariant à chaque palier, sur les 3 stratégies de pagination.

**Critères de sortie — atteints**
- ✅ L'invariant tourne sur **tous** les scénarios (aucune configuration par scénario), avec
  relâchement automatique quand l'`onError` de la sortie autorise des pertes.
- ✅ Vérifié à 10 000 records en réponse unique, et à 3 000 records sur `page`, `offset` et `cursor`
  (scénarios `volume-*`). 10 000 records de bout en bout prennent ~1 s.
- ✅ Régression détectée par l'invariant seul : en tronquant la boucle cursor, `volume-cursor` échoue
  sur `reconcile input total (fetched=1000, expected=3000)` alors que sa seule assertion explicite est
  `log_not_contains panic`. Les invariants *internes* restaient verts (1000 entrés → 1000 sortis est
  cohérent), ce qui démontre que le total déclaré est indispensable.

**Acquis non prévus**
- Le check `sent->wire` a dû devenir **unilatéral** (`wire >= sent`) : un retry rejoue tout le batch,
  donc la livraison est *at-least-once* et le fil reçoit plus que « sent ». Les scénarios retry
  rapportent désormais `replayed=N`, ce qui **quantifie la duplication** — matière directe pour P3.
- **Bug du harnais corrigé** : `run-pipeline-once.sh` préférait un `./cannectors` à la racine à
  `./bin/cannectors`. Un binaire périmé y masquait tous les rebuilds, donc le lab pouvait valider
  du code obsolète sans le signaler. Le script reconstruit maintenant systématiquement vers un
  chemin canonique (`CANNECTORS_SKIP_BUILD=1` pour l'éviter en CI si besoin).

---

### P1 — Volume et endurance — ✅ **terminé** (soak 12 h × 7 sous-systèmes, PASS)

**Objectif** : savoir ce qui se passe au-delà de 5 records et de 1 seconde.

**Tâches**
- Stub source volumineux : soit WireMock avec un `__files` généré (10 k / 100 k records), soit un
  petit serveur Go dédié (plus réaliste pour le streaming et moins gourmand que WireMock).
- Harnais de *soak* : un pipeline CRON à la minute, laissé tourner **2 h puis 24 h**, avec relevé
  périodique de `pprof` (heap, goroutines) et du nombre de connexions ouvertes.
- Seuils d'échec explicites : croissance du heap et du nombre de goroutines bornée entre T+10 min
  et T+2 h (pas de tendance monotone).
- Mesurer la **dérive du CRON** : écart entre `scheduled_time` et l'exécution réelle sur 24 h.
- Comportement en batch : taille de payload maximale acceptée, mémoire consommée à 100 k records,
  effet de `requestMode: single` sur 10 k records (10 k requêtes — durée, sockets).
- Pool DB sur la durée : `maxOpenConns`/`connMaxLifetimeSeconds` respectés après des heures.

**Harnais livré** : `test-lab/scripts/soak.py` (+ `make test-lab-soak DURATION=24h`). Il échantillonne
RSS, descripteurs, threads et connexions Postgres, mesure la dérive CRON, compte les lignes d'erreur,
puis compare le dernier quart du run au deuxième quart (une croissance au démarrage — pool qui se
remplit, cache qui chauffe — ne compte donc pas comme fuite). Verdict PASS/FAIL en sortie.

**Note d'implémentation** : le binaire n'expose pas de pprof, les mesures viennent donc de `/proc`.
C'est suffisant pour voir une tendance ; brancher `net/http/pprof` sur le CLI donnerait le détail par
allocation — changement runtime volontairement écarté du harnais.

**Premier signal (run court de validation, 121 s)** : 122 exécutions × 1 000 records, RSS plat à
~27,5 Mo, descripteurs et threads constants, dérive CRON p50=0 ms / max=1 ms, 0 erreur. Encourageant,
mais sans valeur sur les fuites lentes — d'où le run de 24 h.

**Run long réalisé (12 h, 7 pipelines en parallèle, cadence 5 s)** — `make test-lab-soak-wide`.
Le run de 24 h sur un seul pipeline a été remplacé par 12 h sur sept sous-systèmes simultanés :
ce qui révèle une fuite est le **nombre de ticks**, pas le temps écoulé, et 12 h à 5 s donnent
8 642 exécutions par pipeline contre 1 440 pour 24 h à la minute. Le compromis assumé est une
moitié de résolution en moins sur la dérive monotone très lente, contre sept fois la surface.

| Sous-système | Pipeline | RSS | fds | threads | dérive CRON max | erreurs |
|---|---|---|---|---|---|---|
| Volume / alloc JSON | `volume-1000` | 1,02× | 1,00× | 1,05× | 0 ms | 0 |
| VM goja par exécution | `filters-script-inline` | 1,01× | 1,00× | 1,06× | 1 ms | 0 |
| Cache LRU + pool HTTP | `http-call-path-merge` | 1,01× | 1,00× | 1,00× | 0 ms | 0 |
| Pool Postgres (lecture) | `db-input-basic` | 1,01× | 1,00× | 1,00× | 2 ms | 0 |
| Écriture + transactions | `db-output-upsert-query-file` | 1,00× | 1,00× | 1,08× | 1 ms | 0 |
| State store sur disque | `state-id` | 1,00× | 1,00× | 1,00× | 3 ms | 0 |
| Chemin XML/SOAP | `soap-polling-v12` | 1,01× | 1,00× | 1,00× | 2 ms | 0 |

**60 494 exécutions cumulées, 0 ligne d'erreur, verdict PASS.** RSS entre 26,9 et 30,5 Mo partout et
plat de bout en bout — `volume-1000` a poussé ~8,6 millions de records sans que la mémoire bouge.
Descripteurs strictement constants. Connexions Postgres 6 → 7 sur 12 h, tous pipelines confondus.
Dérive CRON p50 = 0 ms et max = 3 ms sur 8 642 déclenchements.

**Critères de sortie — atteints**
- ✅ Aucune croissance monotone de la mémoire ni des threads sur 12 h × 7 sous-systèmes.
- ✅ Aucune fuite de connexion (compteur Postgres stable à ±1).
- ✅ Dérive CRON documentée et bornée : max 3 ms sur 60 k exécutions.

**Critères de sortie — restants**
- Un chiffre publié : « records/s soutenu » et « mémoire par 10 k records ».

**Ce que ce run ne prouve pas** — à traiter si le besoin de confiance persiste :
- **Les chemins d'erreur n'ont aucune exposition.** Les sept pipelines ont tourné avec 0 erreur, donc
  une fuite dans le retry, le backoff ou `onError` resterait invisible. Un soak dédié contre un stub
  qui échoue une fois sur dix comblerait ce trou.
- **Le webhook n'est pas couvert.** C'est un serveur, pas un job planifié : le harnais ne sait pas le
  piloter. C'est pourtant le module dont le cycle de vie a été modifié le plus récemment.
- **Pas de pipeline OAuth2 dans le jeu**, donc ni l'expiration ni le rafraîchissement de token sur
  plusieurs heures ne sont observés — alors que le cache de tokens est désormais partagé au niveau
  du processus.
- **La croissance disque n'est pas échantillonnée** : `state-id` a tourné 8 642 fois, mais la taille
  du state store n'est pas relevée.

---

### P2 — Conditions réseau réelles — ✅ **terminé**

**Objectif** : quitter le localhost parfait. Une partie est **gratuite** : WireMock sait déjà simuler
des pannes réseau (`EMPTY_RESPONSE`, `MALFORMED_RESPONSE_CHUNK`, `RANDOM_DATA_THEN_CLOSE`,
`CONNECTION_RESET_BY_PEER`) et de la latence (`fixedDelay`, `logNormalRandomDelay`).

**Tâches**
- Scénarios de **fautes WireMock** : réponse vide, corps tronqué en milieu de flux, connexion coupée,
  données aléatoires puis fermeture — pour chaque input (httpPolling, soapPolling) et chaque output.
  Vérifier qu'on obtient une **erreur claire**, pas un record partiel silencieusement accepté.
- **Latence et jitter** : `fixedDelay` supérieur au `timeoutMs`, et jitter log-normal pour vérifier
  que le retry ne s'emballe pas.
- **TLS** : contrepartie HTTPS avec certificat auto-signé → vérifier que la validation échoue
  proprement, puis avec une CA de confiance → vérifier que ça passe. Tester un certificat expiré.
- **DNS** : endpoint sur un nom inexistant → message d'erreur explicite, pas de blocage.
- Optionnel si besoin de plus de finesse : **toxiproxy** entre le binaire et WireMock (bande passante
  limitée, coupures périodiques, paquets réordonnés).

**Critères de sortie — atteints**
- ✅ 11 scénarios `net-*` : réponse vide, reset TCP, chunk malformé, octets aléatoires puis fermeture,
  JSON tronqué, latence > `timeoutMs`, DNS introuvable, TLS auto-signé — côté source **et** destination.
- ✅ **Aucune panne n'est traitée comme un succès.** Le cas le plus critique est vérifié : un corps JSON
  coupé en plein tableau est rejeté (`unexpected end of JSON input`) au lieu de livrer les records
  partiels déjà reçus.
- ✅ TLS : le runtime **valide bien les certificats** — un certificat auto-signé est rejeté. Le lab
  publie le listener HTTPS de WireMock sur 18443 pour le prouver.

**Défaut corrigé en chemin** : `ClassifyNetworkError` construisait le message des erreurs URL avec
`"URL error: <op> <url>"` en **jetant la cause** (`urlErr.Err`). Un certificat invalide, un reset TCP et
un EOF produisaient donc trois logs identiques — des heures de diagnostic perdues en production. Le
message inclut désormais la cause, et l'URL est assainie (query, fragment **et** identifiants
`user:pass@` retirés) via un helper local, `errhandling` ne pouvant pas importer `httpclient`
(cycle d'import). À noter pour P5 : `httpclient.SanitizeURL` retire la query mais **conserve** les
identifiants — à revoir lors de l'audit des secrets dans les logs.

---

### P3 — Crash, reprise et sémantique de livraison — ✅ **terminé**

**Objectif** : répondre à « que se passe-t-il si ça meurt au mauvais moment ? » — aujourd'hui inconnu.

**Tâches**
- Harnais de *kill* : `SIGKILL` (pas `SIGTERM`) à des instants choisis — pendant le fetch, entre
  deux pages, après l'écriture partielle du batch, pendant la sauvegarde de l'état.
- Vérifier l'intégrité du fichier d'état après chaque crash. Bonne nouvelle : `internal/persistence`
  écrit **déjà** en atomique (fichier temporaire puis `os.Rename`, atomique sur POSIX), donc un crash
  en cours d'écriture ne doit pas produire de JSON tronqué — à confirmer par le test plutôt qu'à
  supposer. Nuance restante : aucun `Sync()` n'est appelé avant le `rename`, donc la **durabilité en
  cas de coupure d'alimentation** (fichier vide ou ancienne version au redémarrage) n'est pas garantie.
  À caractériser, et à corriger seulement si le risque est jugé pertinent pour la cible de déploiement.
- **État corrompu volontairement** : JSON invalide, champs manquants, timestamp futur, `lastId`
  inconnu → le pipeline doit repartir proprement ou échouer explicitement, jamais silencieusement
  repartir de zéro (ce serait une **duplication massive**).
- **Runs concurrents** du même pipeline sur le même `storagePath` → dernier écrivain gagne ?
  verrou ? À caractériser puis documenter.
- **Sémantique de livraison** : après un retry sur un batch partiellement écrit en base, compter les
  doublons. Écrire noir sur blanc la garantie réelle (*at-least-once* très probablement) et la
  documenter côté `cannectors-doc`.

**Critères de sortie — atteints**
- ✅ Aucun crash ne laisse un fichier d'état illisible. `test-lab/scripts/crash.py` envoie un
  **SIGKILL** à trois points (`mid-flight`, `after-fetch`, `after-output`) : l'état est toujours du
  JSON valide (ou absent), jamais tronqué — l'écriture atomique tient. **Zéro record perdu.**
- ✅ État corrompu explicite et testé : 3 scénarios (`state-corrupt-invalid-json`,
  `state-corrupt-wrong-type`, `state-corrupt-empty-file`). Comportement réel = **fail-open** :
  l'état est rejeté, le pipeline continue **sans persistance** (donc relecture complète de la
  source) en journalisant `failed to load state ... continuing without persistence`. Le run
  rapporte quand même `success` : une supervision basée sur le statut ne verrait rien passer.
- ✅ Garantie de livraison documentée dans `cannectors-doc/content/docs/concepts/state-persistence.mdx`
  (section « Delivery guarantee and crashes ») et adossée au harnais.

**Limite méthodologique assumée**
- Le harnais mesure **0 doublon**, ce qui ne prouve **pas** l'exactly-once. La livraison est
  at-least-once par conception : un retry rejoue tout le batch (déjà mesuré en P0 via `replayed=N`).
  La fenêtre de duplication propre au crash (entre livraison et écriture de l'état) existe mais
  dure quelques millisecondes : un kill sur marqueur de log tombe de part et d'autre, jamais dedans.
  La prouver exigerait un hook de test retardant l'écriture d'état — changement runtime écarté ici.
- **Piège de montage évité** : la première mesure donnait « 3 doublons » à chaque point… parce que le
  stub source des scénarios `state-*` est **statique** et ignore `after_id`. Le harnais utilise
  désormais `crash-state-id.yaml`, dont la source honore le curseur ; sinon les doublons mesurés
  étaient un artefact du laboratoire, pas une propriété du runtime.

**Reste hors périmètre de cette passe**
- Runs concurrents du même pipeline sur le même `storagePath` (dernier écrivain gagne ? verrou ?) —
  à caractériser.
- Durabilité sur coupure d'alimentation (pas de `fsync` avant le `rename`) — documentée côté doc
  utilisateur, non testée (demanderait de simuler une perte d'alimentation).

---

### P4 — Durcissement du webhook — ✅ **terminé**

**Objectif** : c'est le seul module qui écoute sur le réseau ; il mérite le traitement le plus dur.
Aujourd'hui il est encore couvert par `verify-webhook.sh` (impératif, grep de logs).

**Tâches**
- Migrer les cas webhook vers des scénarios assertés (nécessite d'étendre `run.py` pour piloter un
  process long : bloc `requests:` décrivant les appels à émettre puis les assertions).
- **Charge** : dépasser `rateLimit` et `queueSize`, saturer `maxConcurrent` — vérifier les 429/503,
  l'absence de perte silencieuse des requêtes acceptées, et le comportement de la file pleine.
- **Payloads hostiles** : corps vide, JSON invalide, corps géant (10 Mo, 100 Mo), `Content-Length`
  mensonger, en-têtes dupliqués, `dataField` absent du payload.
- **HMAC négatifs** : signature absente, invalide, algorithme inattendu, corps modifié après
  signature, attaque par rejeu (même signature deux fois — accepté ? à caractériser).
- **Lenteur client** (*slow-loris*) : corps envoyé octet par octet → `requestTimeoutMs` doit couper.
- **Non-régression 22.7** : vérifier que le handler asynchrone ne subit pas l'annulation du contexte
  de la requête HTTP (le bug déjà corrigé) — un test explicite, car c'est un piège structurel.

**Critères de sortie — atteints**
- ✅ Les cas webhook sont désormais des **scénarios déclaratifs** : `run.py` accepte un bloc
  `webhook:` qui démarre le listener, attend le port, envoie les requêtes et vérifie chaque code
  de réponse. Le runner **calcule les HMAC lui-même** (pas de constantes hex), ce qui rend honnêtes
  les cas « corps modifié après signature » et « mauvais secret ».
- ✅ Tous les cas hostiles renvoient un code correct **et le listener survit** : chaque scénario se
  termine par une requête valide, donc un serveur tombé fait échouer le test. Corps vide, JSON
  invalide, JSON non-objet, JSON tronqué, `dataField` absent → **400** dans tous les cas.
- ✅ Rafale au-delà de `rateLimit` : sur 20 requêtes, **6 acceptées (202) et 14 en 429** — rien n'est
  silencieusement jeté.
- ✅ Rejeu HMAC documenté : la même requête rejouée est **acceptée** (la signature ne couvre que le
  corps, sans nonce ni horodatage). Verrouillé par un test et documenté côté doc utilisateur avec la
  recommandation d'un écrit idempotent en aval.

**Défaut corrigé — surface exposée**
`validateSignature` comparait la valeur d'en-tête **brute** à un hex nu, sans gérer de préfixe, alors
que l'en-tête par défaut est `X-Hub-Signature-256` — dont la convention (GitHub et assimilés) envoie
**toujours** `sha256=<hex>`. **Un vrai webhook GitHub aurait donc échoué en 401 avec la configuration
par défaut.** Le préfixe est maintenant retiré avant comparaison ; l'hex brut reste accepté
(rétro-compatible), la comparaison reste en temps constant. Documenté dans
`cannectors-doc/.../inputs/webhook/index.mdx`.

**Reste hors périmètre de cette passe**
- Corps géants (10 Mo / 100 Mo), `Content-Length` mensonger, en-têtes dupliqués.
- Client lent (*slow-loris*) contre `requestTimeoutMs`.
- Saturation de `queueSize` / `maxConcurrent` observée en tant que telle (le rejet 429 arrive avant).
- L'ancien `verify-webhook.sh` est conservé : il couvre encore le chemin file d'attente et sert de
  filet pendant la transition.

---

### P5 — Sécurité et vie longue — ✅ **terminé**

**Objectif** : vérifier les deux affirmations que je n'ai pas auditées, et les limites du bac à sable.

**Tâches**
- **Secrets jamais loggés — test sentinelle automatisable** : donner à chaque credential une valeur
  sentinelle unique (`SENTINEL_a1b2c3`), lancer **toute la suite**, puis `grep` la sentinelle dans
  l'intégralité des logs capturés. Zéro occurrence = l'affirmation tient. Coût faible, valeur élevée,
  et c'est rejouable en CI.
- **Garde-fou SQL** : la décision d'`PLAN_TEMPLATING_JINJA.md` confie la correction à l'auteur de la
  config (« query templatée + `parameters` bindés »). Un `{{ record.x }}` écrit **dans le texte** de la
  query est donc rendu comme du texte — par conception. Deux actions : (a) un test qui **documente**
  ce comportement avec une valeur contenant `'; DROP TABLE`, pour qu'il soit visible et non découvert
  en production ; (b) un avertissement au `validate` (ou une entrée de lint) quand une query référence
  `record.*` hors `parameters`.
- **SSRF** : endpoint construit depuis une donnée de record (`{{ record.url }}`) → est-il possible de
  viser `169.254.169.254` ou `localhost` ? Caractériser, puis documenter la recommandation.
- **Bac à sable script (Goja)** : boucle infinie (l'`Interrupt` est branché sur le contexte — vérifier
  qu'il coupe réellement, et sous quel délai), allocation mémoire massive, tentative d'accès I/O
  (`require`, `fetch`, `process`), récursion profonde.
- **OAuth2, expiration et refresh** : contrepartie délivrant un token `expires_in: 2` → vérifier le
  renouvellement, et le comportement quand le refresh échoue. Jamais testé (mes tests obtiennent un
  token une seule fois).

**Critères de sortie — atteints, sauf un**
- ✅ **Audit sentinelle** : `test-lab/scripts/secret-audit.py` donne à chaque emplacement de
  credential une valeur unique, passe le pipeline par `validate`, `validate --verbose`,
  `run --dry-run`, `run` et `run --verbose`, puis cherche ces valeurs dans toute la sortie.
  **Verdict PASS après correctif** (voir ci-dessous).
- ✅ **Templating SQL testé** : `db-output-sql-injection` envoie
  `Robert'); DROP TABLE dest_customers; --` comme valeur de record. La table survit et la valeur est
  stockée **verbatim** — les `parameters` sont bien liés, jamais interpolés.
- ✅ **Bac à sable étanche** : `script-sandbox-isolation` vérifie que `require`, `fetch`, `process`,
  `XMLHttpRequest` et `globalThis.fs` sont **tous indéfinis**. Aucune évasion I/O.
- 🔴 **Un script hostile PEUT bloquer indéfiniment** — voir le finding ci-dessous. Non corrigé.
- 🟡 Le refresh OAuth2 « fonctionne » mais ne peut pas être distingué : il n'y a **aucun cache**.

**Correctif — fuite de credentials dans les logs**
L'audit a trouvé une vraie fuite : des identifiants placés dans l'URL
(`http://user:password@host/…`) étaient journalisés **en clair** sur `run`, `run --dry-run` et
`run --verbose`. Cause racine unique : `httpclient.SanitizeURL` retirait la query et le fragment mais
**conservait le userinfo**. Corrigé par `parsed.User = nil`, ce qui assainit d'un coup les **57 sites
d'appel**. Tests ajoutés dans `internal/httpclient/error_test.go`. Tous les autres emplacements
étaient déjà propres : bearer, basic, api-key, secret OAuth2, token en query, mot de passe DSN.

**✅ FINDING CORRIGÉ — un script emballé rendait le process inarrêtable**
*(description d'origine conservée ci-dessous ; correctif au bout)*
Un filtre `script` contenant `while (true) {}` (idem allocation massive ou récursion infinie) :
- tourne indéfiniment en consommant **~91 % d'un cœur** ;
- **reçoit** SIGTERM (« Received terminated signal ») mais **n'achève jamais son arrêt** :
  le scheduler journalise `cron stop context timeout - continuing with shutdown` et le process est
  toujours vivant 6 s plus tard — **SIGKILL obligatoire**.
En production : `systemctl stop` qui traîne, `docker stop` qui attend sa période de grâce, déploiements
progressifs bloqués. Le mécanisme d'interruption existe pourtant (`context.AfterFunc(ctx, …)` →
`runtime.Interrupt` dans `internal/modules/filter/script.go`) et le scheduler appelle bien `s.cancel()`
— **le contexte reçu par le filtre n'est donc pas celui qui est annulé**. Tracer et corriger ce chemin
touche l'arrêt et l'exécution : délibérément laissé au user plutôt qu'improvisé.
**Cause racine trouvée** : `PipelineExecutorAdapter` (cmd/cannectors/main.go) n'implémentait que
`Execute(pipeline)`. Le scheduler fait `s.executor.(ContextExecutor)` et **retombe silencieusement**
sur `Execute` quand l'assertion échoue — or `Executor.Execute` appelle
`ExecuteWithContext(context.Background(), …)`. **Toute exécution planifiée tournait donc sous un
contexte non annulable.** Le mécanisme d'interruption fonctionnait parfaitement (prouvé par un test
unitaire isolé) : il ne recevait simplement jamais l'annulation.

**Correctif** : l'adaptateur implémente `ExecuteWithContext` et propage le contexte jusqu'au runtime ;
`Execute` délègue avec `context.Background()` pour compatibilité. Une **assertion de compilation**
(`var _ scheduler.ContextExecutor = (*PipelineExecutorAdapter)(nil)`) empêche ce mode dégradé
silencieux de revenir.

**Résultats** : le script emballé s'arrête en **105 ms** après SIGTERM (au lieu de jamais).
**Bonus non prévu** : le trace ID du scheduler se propage enfin dans le runtime — les lignes
`scheduled pipeline execution starting` et `execution started` portaient jusqu'ici des IDs
**différents**, ce qui cassait la corrélation des logs. Régression verrouillée par
`internal/modules/filter/script_interrupt_test.go`.

**✅ FINDING CORRIGÉ — aucun cache des tokens OAuth2**
Un token annoncé valide **3600 s** est redemandé **à chaque exécution** (8 demandes pour 8 appels,
mesuré). Un pipeline pollant toutes les 15 s ferait ~5 760 demandes de token par jour pour rien —
beaucoup de fournisseurs (Auth0, Okta…) limitent agressivement ce endpoint. Conséquence pour les tests :
le scénario `auth-oauth2-refresh` prouve que l'authentification tient dans la durée, mais **ne peut pas
distinguer** « renouvelé à l'expiration » de « redemandé systématiquement ».

**Cause racine — plus large que l'OAuth2** : le cache de tokens existait bien (`cachedToken`,
`tokenExpiryBuffer`) mais **par instance de handler**, et *chaque tick reconstruit tous les modules*
(`factory.Create*Module` dans l'adaptateur). Tous les caches repartent donc froids à chaque exécution.
Mesuré au-delà de l'OAuth2 : le **cache LRU d'`http_call` est lui aussi réinitialisé** — 18 appels
d'enrichissement pour 6 exécutions, soit 3 par tick au lieu de 3 au total.

**Correctif (ciblé)** : `internal/auth/oauth2_tokencache.go` ajoute un cache de tokens **à l'échelle
du process**, clé = SHA-256 de (tokenUrl, clientId, clientSecret, scopes) — un secret pivoté produit
donc une clé différente, et aucun secret n'est conservé lisible en mémoire. Mesure : **8 exécutions →
1 seule demande de token** (contre 8 avant). Le renouvellement à expiration reste actif (vérifié avec
`expires_in: 1`). Tests dans `oauth2_tokencache_test.go`.

**🟡 RESTE OUVERT — les modules sont reconstruits à chaque tick**
Le correctif ci-dessus traite le symptôme le plus coûteux (endpoint de token martelé, souvent
rate-limité). La cause structurelle demeure : à chaque exécution planifiée, les modules input/filter/
output sont recréés, ce qui réinitialise les caches LRU d'`http_call`/`sql_call` et fait tourner les
pools de connexions. Corriger cela (construire les modules une fois par pipeline et les réutiliser)
est un changement d'architecture — état des modules, sûreté concurrente, sémantique de `Close` — qui
mérite sa propre branche et sa propre revue.

*Note : ce comportement n'est pas démontrable par un scénario déclaratif, le runner tuant le pipeline
après la première exécution. Preuve reproductible à la main :
`timeout 8 ./bin/cannectors run test-lab/pipelines/auth-input-oauth2.yaml` puis compter les requêtes
dans le journal WireMock.*

---

### P6 — Casser les angles morts corrélés — ✅ **terminé**

**Objectif** : trouver ce qu'une lecture du code ne peut pas révéler, parce que les tests ont été
écrits d'après ce même code.

**Tâches**
- **Fuzzing** (`go test -fuzz`) sur : le parseur YAML/JSON de config, `internal/sqltemplate`, le
  moteur de templates, et les transforms de `mapping`. Objectif : aucun panic, aucune boucle infinie.
- **Tests de propriété** sur les transforms : par exemple `split` puis `join` avec le même séparateur
  est l'identité pour toute chaîne sans espaces en bordure ; `toString(toInt(s)) == s` pour tout entier
  valide. Ces propriétés valent pour des milliers d'entrées, là où mes cellules en couvrent une chacune.
- **Assertions dérivées de la doc, pas du code** : reprendre `cannectors-doc` comme *seule* source et
  écrire des scénarios depuis les promesses documentées. C'est ce qui a révélé le gap `${VAR}` — méthode
  à systématiser (elle a déjà prouvé son rendement).

**Critères de sortie — atteints**
- ✅ **Fuzzing en CI** : 7 cibles lancées 30 s chacune à chaque push
  (`.github/workflows/ci.yml`), avec remontée du corpus fautif en artefact si une entrée casse.
  Localement : 251 k exécutions sur le parseur de config, 300 k sur le compilateur SQL, 100 à 240 k
  par propriété — **aucun crash, aucun blocage**.
- ✅ **6 propriétés vérifiées** sur les transforms (`mapping_properties_test.go`) : idempotence de
  `trim`, idempotence de `uppercase`/`lowercase`, aller-retour `split`/`join` (à trim près),
  aller-retour entier via `toString`/`toInt`, équivalence stricte de `toInt` avec `strconv.Atoi`,
  `toArray` qui n'enveloppe qu'une fois, `replace` sans correspondance = identité.
- ✅ **Passe « doc → tests »** sur les pages pagination et authentification.

**Ce que la méthode a produit**
- Une **propriété fausse de ma part**, attrapée immédiatement : j'avais posé
  `uppercase(lowercase(s)) == uppercase(s)`. Faux en Unicode — « İstanbul » minusculisé donne un
  « i » suivi d'un point combinant, qui remajusculise en « ISTANBUL ». Idem pour ß → SS. C'est le
  pliage de casse standard de Go, pas un défaut ; la propriété a été remplacée par l'idempotence,
  qui est vraie, et le piège est documenté dans le test.
- **Trois promesses documentées sans aucun test**, désormais couvertes :
  `limit` sans `limitParam` est silencieusement ignoré ; `totalPagesField` accepte un chemin pointé
  (seuls `dataField` et `nextCursorField` l'étaient) ; un `nextCursorField` pointant sur un booléen
  vaut « pas de curseur » et arrête la boucle — exactement le garde-fou choisi en retirant la
  coercition des booléens.
- **Confirmation de la valeur de la méthode** : la page authentification affirme que Cannectors
  « acquires a token on first use, **caches it** ». Cette phrase était **fausse jusqu'au correctif
  P5**. Une passe « doc → tests » menée plus tôt aurait pointé le trou directement — c'est le même
  mécanisme qui avait révélé l'absence de substitution `${VAR}`.

---

### P7 — Pilote réel en mode shadow — **effort M, rendement décisif**

**Objectif** : c'est le seul item qui ferme l'écart « WireMock ≠ réalité ». Il aurait attrapé le bug du
curseur numérique le premier jour.

**Tâches**
- Choisir **un** pipeline non critique contre une vraie API (idéalement paginée et authentifiée).
- Le faire tourner en **shadow** : lecture réelle, écriture vers une destination jetable.
- **Réconcilier** : nombre d'enregistrements côté source (via l'API elle-même) contre le nombre reçu,
  quotidiennement, sur 2 semaines.
- Observer ce qui n'existe pas en laboratoire : formes de pagination inattendues, rate limits, 5xx
  transitoires, dérive d'horloge, tokens qui expirent, champs qui changent de type entre deux appels.
- Écrire un runbook minimal : comment on détecte un incident, comment on rejoue.

**Critères de sortie**
- 14 jours consécutifs avec réconciliation exacte.
- Tout écart observé transformé en scénario de laboratoire (c'est le vrai livrable).

---

## 4. Ordre recommandé

```
P0 (invariant)  →  P1 (volume/endurance)  →  P7 (pilote shadow, en parallèle dès que P0 est là)
                →  P2 (réseau)  →  P3 (crash/livraison)  →  P4 (webhook)  →  P5 (sécurité)  →  P6 (fuzz/propriétés)
```

P0 d'abord parce qu'il rend tout le reste plus détectable. P7 peut démarrer en parallèle dès que
l'invariant existe : c'est le seul apport d'information vraiment nouvelle.

---

## 5. Indicateurs à suivre

| Indicateur | Aujourd'hui | Cible |
|---|---|---|
| Scénarios couverts par l'invariant `N in = N out` | 0 % | ≥ 90 % |
| Volume maximal validé | 5 records | 100 000 |
| Durée de run validée | ~1 s | 24 h |
| Types de pannes réseau testés | 0 | ≥ 6 |
| Scénarios de crash/reprise | 0 | ≥ 8 |
| Webhook : cas assertés déclarativement | 0 | ≥ 15 |
| Jours de pilote réel réconciliés | 0 | 14 |

---

## 6. Ce qui restera inconnu même après tout ça

À énoncer d'avance pour ne pas se raconter d'histoires :

- Le comportement face à **l'API spécifique** de chaque futur utilisateur, avec ses propres écarts au standard.
- La montée en charge **multi-pipelines concurrents** sur une même instance (non traité ici).
- La sécurité d'une exposition **publique** du webhook (pas d'audit externe, pas de pentest).
- Les **régressions de performance** dans le temps (il faudrait un benchmark en CI avec seuils).

Une fois P0 à P7 faits, la question « digne de confiance en production surveillée » passerait
raisonnablement de **~40 %** à **~80-85 %**. Le dernier segment ne s'obtient pas par des tests :
il s'obtient par du temps de vol réel et par des incidents traités.
