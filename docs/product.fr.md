# VideoCMS — Documentation produit

> **Langues :** [English](product.md) | [中文](product.zh-CN.md) | Français | [日本語](product.ja.md) | [Deutsch](product.de.md)

## 1. Qu’est-ce que VideoCMS ?

VideoCMS est un système auto-hébergé de gestion de ressources vidéo. Pointez-le vers
des dossiers de votre disque, lancez une analyse, et toutes vos vidéos deviennent une
médiathèque consultable et rechercheable — avec affiches, métadonnées, progression de
lecture, favoris, listes de lecture et une catégorie « Séries TV » pour les épisodes numérotés.

Il est construit avec **Go + React + PostgreSQL** et fonctionne entièrement sur votre
matériel. Vos vidéos ne quittent jamais votre réseau, sauf si vous choisissez de les partager.

## 2. Fonctionnalités

| Domaine | Ce que vous obtenez |
| --- | --- |
| Médiathèques | N’importe quel dossier serveur ; ajout par chemin ou avec le sélecteur de dossiers intégré |
| Analyse | Découverte automatique de mp4/mkv/webm/avi/mov/ts… avec progression en direct et annulation |
| Métadonnées | Codec/résolution/durée via ffprobe, affiches générées depuis la vidéo, titre/année/synopsis/genres modifiables |
| Scraping TMDB | Enrichissement en ligne facultatif : titres localisés, synopsis, genres et affiches |
| Lecture | Lecture native H.264/WebM dans le navigateur ; transcodage HLS automatique pour MKV/HEVC |
| Séries TV | Fichiers numérotés (S01E01, EP1, 第1集…) regroupés automatiquement et triés par épisode |
| Personnel | Reprendre la lecture, favoris, listes de lecture avec lecture séquentielle |
| Utilisateurs | Inscription/connexion, rôles administrateur, gestion des utilisateurs |
| Interface | 5 langues : anglais (défaut), 中文, Français, 日本語, Deutsch |

## Captures d’écran

> *À venir — lancez `make serve` et ouvrez `http://<ip-serveur>:8080` pour voir
> l’interface en action.*

## 3. Démarrage rapide

### Prérequis

- Un serveur (macOS/Linux/Windows) avec Go 1.22+ pour compiler (ou un binaire précompilé)
- PostgreSQL 14+
- ffmpeg/ffprobe (métadonnées, affiches, transcodage)
- Node.js 18+ (uniquement pour le développement front ; l’UI est servie par le backend)

### Installation

```bash
createdb videocms                       # ou : docker compose up -d db
cd backend && go run ./cmd/server       # premier démarrage : tables + admin/admin123
cd frontend && npm install && npm run dev   # http://localhost:5173
```

Pour un accès LAN / téléphone, compilez une fois et utilisez un seul port :

```bash
make serve                              # http://<IP-LAN>:8080
```

### Première connexion

Ouvrez l’interface et connectez-vous avec l’administrateur initial **admin / admin123**
(changez le mot de passe immédiatement : Administration → Utilisateurs → Réinitialiser).

### Ajouter votre première médiathèque

1. Allez dans **Administration → Bibliothèques**
2. Saisissez un nom, puis un chemin serveur ou cliquez sur **Parcourir…** pour choisir
3. Cliquez sur **Analyser** — le compteur progresse en direct ; les vidéos apparaissent sur l’accueil

## 4. Guide utilisateur

### 4.1 Parcourir et rechercher

- L’accueil affiche Reprendre la lecture, Séries TV et la grille de vidéos
- La recherche porte sur le titre, le synopsis et le nom de fichier
- Filtres par bibliothèque et type (Tout / Films / Séries TV), tri par titre,
  année, durée, date d’ajout ou popularité
- « Charger plus » pagine la grille

### 4.2 Lire des vidéos

- Cliquez sur une vidéo pour la page de détail (affiche, métadonnées, synopsis)
- **Lecture** démarre ; si vous avez de la progression, elle reprend automatiquement
- H.264 MP4 / WebM se lisent nativement ; MKV/HEVC sont transcodés à la volée
  (première lecture : quelques secondes ; « Lecture transcodée » en secours)
- Le téléchargement est disponible pour une lecture hors ligne
- La progression est sauvegardée toutes les 5 secondes et à la pause/fin

### 4.3 Favoris et listes de lecture

- ☆ Favori sur la page de détail ; gérez-les dans « Mes favoris »
- Créez des listes depuis la page Listes de lecture ou depuis toute vidéo
  (« + Ajouter à la liste »)
- Une liste se lit dans l’ordre (▶ Tout lire) avec une file visible

### 4.4 Séries TV

Les vidéos portant des marqueurs d’épisode dans leur nom sont regroupées automatiquement
après une analyse. Formats pris en charge : `S01E01`, `E01`, `EP1`, `第1集`, nombres
en fin de nom (`1 (4)`, `-535`). Un groupe doit contenir au moins 2 épisodes.

- La page **Séries TV** liste toutes les séries avec affiches et nombre d’épisodes
- Une page de série affiche les épisodes triés saison → épisode
- Les séries sont reconstruites à chaque analyse ; celles avec moins de 2 épisodes
  disponibles sont nettoyées automatiquement

### 4.5 Astuces de lecture

- Le lecteur utilise les contrôles natifs du navigateur — **Espace** lecture/pause,
  **F** plein écran, **←/→** avance/retour, **↑/↓** volume, **M** muet
- La progression reprend automatiquement ; « Reprendre la lecture » vous ramène
  à la position précédente
- Les MKV/HEVC sont transcodés en HLS — la première lecture prend quelques
  secondes, puis le défilement et la lecture automatique de l’épisode suivant
  fonctionnent normalement
- Sur mobile, passez en paysage pour une meilleure expérience ; utilisez la file
  sous le lecteur pour changer d’épisode

## 5. Guide administrateur

### 5.1 Aperçu

Statistiques : vidéos, bibliothèques, utilisateurs, listes, favoris, séries et espace utilisé.

### 5.2 Bibliothèques

- Ajout avec un nom et un chemin serveur (sélecteur de dossiers inclus)
- **Analyser** indexe les nouveautés ; **Arrêter l’analyse** annule ; progression en direct
- Supprimer une bibliothèque retire ses enregistrements — les fichiers restent sur le disque

### 5.3 Vidéos

- Recherchez une vidéo et modifiez titre/année/genres/synopsis
- Importez une affiche personnalisée
- **Récupérer** obtient les métadonnées de TMDB (nécessite `TMDB_API_KEY` et l’accès
  réseau à api.themoviedb.org)

### 5.4 Utilisateurs

- Changez les rôles (utilisateur/admin), réinitialisez les mots de passe, supprimez des comptes
- Garde-fous : impossible de supprimer son propre compte ; le dernier admin ne peut
  être ni supprimé ni rétrogradé

## 6. Configuration

Tout se configure par variables d’environnement (table complète dans le README). Principales :

| Variable | Défaut | Rôle |
| --- | --- | --- |
| `PORT` | `8080` | Port d’écoute |
| `DATABASE_URL` | `postgres://localhost:5432/videocms` | Connexion base de données |
| `JWT_SECRET` | valeur dev | **Secret fort obligatoire en production** |
| `TMDB_API_KEY` | vide | Active le scraping de métadonnées |
| `SCAN_WORKERS` | `4` | Workers d’analyse parallèles |

## 7. FAQ

**Pourquoi ma bibliothèque reste « Analyse en cours » ?**
La première analyse d’un grand dossier prend du temps (ffprobe + affiche par fichier).
La progression est en direct et vous pouvez annuler à tout moment. Les analyses
suivantes ne traitent que les fichiers modifiés. Les fichiers `._` de macOS et les
dossiers de segments `.m3u8` sont ignorés automatiquement.

**Pourquoi mon navigateur ne lit pas un MKV/HEVC ?**
Les navigateurs ne décodent que H.264/VP9. VideoCMS transcode MKV/HEVC en HLS à la
volée, ou vous pouvez télécharger le fichier pour une lecture locale.

**Comment les séries sont-elles détectées ?**
Fichiers dont le nom contient `S01E01`, `E01`, `EP1`, `第1集` ou des nombres finaux,
regroupés par préfixe commun. Au moins 2 épisodes par groupe. Si votre schéma de
nommage n’est pas reconnu, adaptez les noms ou demandez un nouveau motif.

**Le scraping TMDB échoue.**
Vérifiez `TMDB_API_KEY` et l’accès à api.themoviedb.org. Sur un réseau restreint,
seul le scraping est indisponible ; tout le reste fonctionne.

**Comment accéder à VideoCMS depuis mon téléphone ?**
Les deux appareils doivent être sur le même réseau. Lancez `make serve`, trouvez
l’IP avec `ipconfig getifaddr en0`, ouvrez `http://<ip>:8080`. Autorisez le
pare-feu macOS à la première exécution.

**Est-ce sûr en accès public ?**
Le déploiement par défaut est en HTTP non chiffré avec un secret JWT de développement.
Pour tout accès hors LAN de confiance, utilisez un proxy inverse HTTPS et définissez `JWT_SECRET`.

## 8. Technologies et licence

Go + React + PostgreSQL, avec ffmpeg pour le traitement média. Interface entièrement
localisée (en/zh/fr/ja/de). Sous licence **Apache License 2.0**.

Pour les aspects internes, voir [l’architecture système](architecture.md).
