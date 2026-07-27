# Plan de validation — passer du « vert en laboratoire » au « digne de confiance en production »

> Statut : **P0 terminé**, P1 à P6 à faire. Détail par phase ci-dessous.
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

### P1 — Volume et endurance — **effort M, rendement élevé**

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

**Critères de sortie**
- 24 h de run sans croissance monotone du heap ni des goroutines.
- Aucune fuite de connexion (compteur stable côté Postgres/MySQL).
- Dérive CRON documentée et bornée.
- Un chiffre publié : « records/s soutenu » et « mémoire par 10 k records ».

---

### P2 — Conditions réseau réelles — **effort M, rendement élevé**

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

**Critères de sortie**
- Chaque type de panne produit un `status: error` avec un message identifiant la cause.
- **Aucun cas où une réponse tronquée est traitée comme un succès** (c'est le risque majeur ici).
- Comportement TLS documenté (validation active par défaut).

---

### P3 — Crash, reprise et sémantique de livraison — **effort M, rendement élevé**

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

**Critères de sortie**
- Aucun crash ne laisse un fichier d'état illisible.
- Le comportement sur état corrompu est explicite et testé.
- La garantie de livraison est écrite dans la doc et adossée à un test.

---

### P4 — Durcissement du webhook — **effort M, rendement élevé (surface exposée)**

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

**Critères de sortie**
- Aucune requête acceptée (2xx) n'est perdue sans trace.
- Tous les cas hostiles renvoient un code correct sans faire tomber le serveur.
- Le rejeu HMAC a un comportement documenté.

---

### P5 — Sécurité et vie longue — **effort S à M, rendement élevé sur deux points précis**

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

**Critères de sortie**
- La sentinelle secrète n'apparaît nulle part dans les logs, en CI.
- Le comportement du templating SQL est testé et documenté (pas laissé implicite).
- Un script hostile ne peut ni bloquer indéfiniment ni sortir du bac à sable.
- Le refresh OAuth2 est prouvé.

---

### P6 — Casser les angles morts corrélés — **effort M, rendement moyen mais unique**

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

**Critères de sortie**
- Corpus de fuzzing en CI, sans crash.
- ≥ 5 propriétés vérifiées sur les transforms.
- Une passe complète « doc → tests » sur au moins deux pages (auth, pagination).

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
