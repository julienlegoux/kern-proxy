# CONVENTIONS.md — kern-link

Autorité locale pour ce repo, comme annoncé par le [CONTRIBUTING.md](https://github.com/kern-ia/.github/blob/main/CONTRIBUTING.md)
de l'organisation. Les règles communes à tous les repos `kern-ia` sont reprises ci-dessous ;
la section « Spécificités » couvre ce qui n'appartient qu'à `kern-link`.

`kern-link` est aujourd'hui le seul repo de l'org qui applique déjà un vrai flux de Pull
Request GitHub (51 PR mergées à ce jour) — il sert de référence pour les autres repos plutôt
que l'inverse sur ce point précis.

## Branches

- `main` : branche stable, toujours déployable. Protégée — aucun push direct.
- Branche d'intégration : **`dev`** — norme commune à tout `kern-ia` (voir tous les autres
  `CONVENTIONS.md` de l'org).

> **À corriger sur ce repo** : la branche d'intégration s'appelle aujourd'hui `develop`, seul
> repo de l'org dans ce cas. Renommer `develop` → `dev` pour s'aligner (opération à faible
> risque : renommer la branche, mettre à jour sa protection et la base des PR ouvertes) — pas
> une question à trancher, juste une correction à faire, volontairement et pas en silence.

- Branches de travail : `feature/<slug>`, `fix/<slug>`, `chore/<slug>`, `docs/<slug>`,
  `release/<version>` (déjà l'usage réel ici : `repo-polish`, `rename-kern-link`…).
- Toute modification de `main` ou `develop` passe par une Pull Request — déjà respecté.
- Merge : merge commit standard via le bouton GitHub (`Merge pull request #N from
  <owner>/<branche>`) — pattern déjà en place, à documenter comme référence pour les autres
  repos qui font aujourd'hui des merges locaux invisibles sur GitHub.

## Commits

Conventional Commits : `feat:`, `fix:`, `chore:`, `docs:`, `chore(release):`… Aucune signature
d'outil (trailer `Co-Authored-By`, `Claude-Session` ou équivalent) dans les messages de
commit — l'auteur du commit git suffit. Les commits passés de ce repo en portent encore ;
ne pas en ajouter de nouveaux, pas besoin de réécrire l'historique.

## Pull Requests

- Un seul sujet par PR, liée à l'issue ou la RFC qu'elle résout.
- Template PR hérité de `kern-ia/.github`.
- Déclare l'impact semver.
- Aucune donnée personnelle réelle.

## Style et lint

`.golangci.yml` — `version: 2`, `linters.default: standard`, volontairement minimal (le
commentaire en tête du fichier l'assume : élargir le set est un futur travail délibéré, pas un
oubli). Deux narrowings documentés (`errcheck` sur `io.Closer.Close`/`fmt.Fprint*`,
`staticcheck` sans `ST1005`) — chacun porte sa justification inline, à garder comme modèle
pour documenter toute future exception de lint dans n'importe quel repo de l'org.
`max-issues-per-linter: 0`, `max-same-issues: 0`.

## Tests / CI

`.github/workflows/test.yml` : jobs `test` (`go test ./... -race -v` + test offline du script
de sync upstream) et `lint` (`golangci-lint`). `.github/workflows/upstream-sync.yml` gère la
synchronisation hebdomadaire avec le projet amont porté — spécificité propre à ce repo (port
de `@earendil-works/pi-ai`), pas un pattern à généraliser ailleurs dans l'org.

## Module Go

- Chemin actuel : `github.com/julienlegoux/kern-link` — cohérent avec le compte GitHub de
  l'auteur principal du repo, mais diverge du chemin `github.com/kern-ia/...` qu'on
  attendrait pour un repo hébergé sous l'organisation. Même décision à trancher au niveau org
  que pour `kern-ui`/`kern-orch`/`kern-anon` (voir rapport global) — ne pas renommer
  unilatéralement ici, c'est un module consommé en aval.

## Release / CHANGELOG

> **Écart avec la politique org** : `CONTRIBUTING.md` de `kern-ia/.github` dit explicitement
> « il n'y a pas de `CHANGELOG.md` » et que les notes de version vivent dans le tag annoté.
> `kern-link` maintient pourtant un `CHANGELOG.md` réel et à jour, avec un usage cohérent
> (releases v0.1.0, v0.1.1 documentées). Deux issues possibles : soit `kern-link` reste une
> exception documentée (assumé, à noter explicitement dans le CONTRIBUTING.md org), soit
> l'org généralise ce pattern à tous les repos. À trancher, pas à laisser en silence.

## Documentation

- `README.md`, `LICENSE`, `NOTICE` à la racine — `NOTICE` est spécifique à ce repo (attribution
  du projet amont porté).
- Pas de `CLAUDE.md` aujourd'hui, contrairement à `kern-ui`/`kern-orch`/`kern-anon` — à créer
  si des sessions Claude Code doivent intervenir régulièrement sur ce repo.

## Sécurité / confidentialité

Voir `SECURITY.md` hérité de l'org. Attention particulière ici : `kern-link` reçoit des
identifiants éphémères par appel (jamais de credentials persistés dans le repo).
