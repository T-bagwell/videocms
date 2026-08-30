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
| Blocage de contenu | Les administrateurs bloquent des médias par titre — masqués pour tous, fichiers et enregistrements conservés, déblocage à tout moment |
| Interface | 5 langues : anglais (défaut), 中文, Français, 日本語, Deutsch |

## Captures d’écran

> *À venir — lancez `make serve` et ouvrez `http://<ip-serveur>:8080` pour voir
> l’interface en action.*

## 3. Démarrage rapide

### Prérequis

- Un serveur (macOS/Linux/Windows) avec Go 1.26+ pour compiler (ou un binaire précompilé)
- PostgreSQL 14+
- ffmpeg/ffprobe (métadonnées, affiches, transcodage)
- Node.js 20+ (uniquement pour le développement front ; l’UI est servie par le backend)

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
2. Saisissez un nom, puis un chemin serveur **absolu** (p. ex. `/media/movies`)
   ou cliquez sur **Parcourir…** pour choisir — les chemins relatifs sont refusés
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
- Les vidéos à plusieurs pistes audio permettent de changer de piste pendant
  la lecture HLS (sélecteur audio dans le lecteur)
- Les sous-titres peuvent être décalés de ±0,5 s pour corriger la synchronisation
  en lecture directe ; le décalage est enregistré par utilisateur et par vidéo
- En survolant la barre de progression, des aperçus vidéo s’affichent pendant
  le déplacement ; un clic saute à la position visée
- Les sous-titres ASS/SSA sont rendus avec leur style d’origine (polices,
  couleurs, position et effets)
- Les administrateurs peuvent rechercher et télécharger des sous-titres auprès
  de fournisseurs en ligne (p. ex. OpenSubtitles) par vidéo
- **Regarder ensemble** : créez ou rejoignez une salle pour synchroniser la
  lecture avec vos amis ; un bouton **Diffuser / AirPlay** est disponible si
  le navigateur le prend en charge
- **Direct** : les administrateurs créent des flux RTMP (URL d’ingestion
  compatible OBS) et les spectateurs profitent d’un tchat intégré
- Les administrateurs peuvent lancer la **transcription vocale** (Whisper)
  d’une vidéo ; le texte est recherchable et sélectionnable comme piste de
  sous-titres
- **Télécharger** un remux MKV ou MP4 avec la piste audio et les sous-titres
  choisis (sans ré-encodage), ou récupérer le fichier original
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
- La lecture transcodée utilise une échelle multi-qualité adaptative — utilisez
  le sélecteur de qualité au-dessus du lecteur pour changer (Auto par défaut)
- Les sous-titres (fichiers à côté, intégrés ou téléversés) peuvent être activés
  ou désactivés dans le lecteur, changés de langue quand plusieurs pistes existent,
  et chaque utilisateur peut définir son propre défaut par vidéo (les admins
  peuvent définir le défaut global)
- Sur mobile, passez en paysage pour une meilleure expérience ; utilisez la file
  sous le lecteur pour changer d’épisode

### 4.6 Partage

- Ouvrez une vidéo, une série ou une liste de lecture et appuyez sur **Partager**
  pour créer un lien (7 jours par défaut, de 1 heure à 1 an)
- Toute personne ayant le lien peut regarder sur une page publique — aucun
  compte n’est nécessaire ; les liens de série et de liste jouent toute la file
- Les liens expirent automatiquement et peuvent être révoqués à tout moment
  depuis la même fenêtre
- Les liens peuvent être protégés par mot de passe — le mot de passe est demandé
  sur la page publique avant le chargement du contenu
- Les liens peuvent être limités à une liste de domaines autorisés — les
  requêtes depuis d’autres hôtes sont rejetées
- Le contenu partagé respecte les contrôles d’administration : les titres et
  bibliothèques bloqués n’apparaissent jamais dans les liens de partage

## 5. Guide administrateur

### 5.1 Aperçu

Statistiques : vidéos, bibliothèques, utilisateurs, listes, favoris, séries et espace utilisé.

- **Export / import de sauvegarde** : téléchargez une sauvegarde JSON complète
  des métadonnées du serveur, ou restaurez-en une (bibliothèques et vidéos sont
  fusionnées par chemin ; les données personnelles sont restaurées pour les
  utilisateurs existants)

### 5.2 Bibliothèques

- Ajout avec un nom et un chemin serveur **absolu** (sélecteur de dossiers inclus)
- Onglet **Téléversements** : téléversements en morceaux reprenables vers tout
  dossier serveur (p. ex. un dossier de bibliothèque) — indexés automatiquement
  une fois terminés
- **Analyser** indexe les nouveautés ; **Arrêter l’analyse** annule ; progression en direct
- Supprimer une bibliothèque retire ses enregistrements — les fichiers restent sur le disque
- **Ouvrir le dossier** ouvre le répertoire de la médiathèque sur le serveur
  avec le gestionnaire de fichiers système, pour inspecter ou gérer les médias
- **Contrôle de santé** signale fichiers manquants/corrompus et doublons ;
  **Garder la meilleure version** déplace le reste vers la corbeille du serveur
- **Exporter NFO / Importer NFO** lit et écrit les métadonnées Kodi à côté des
  vidéos (compatible Plex/Jellyfin/Kodi)
- **Bloquer la bibliothèque** masque toute la médiathèque pour chaque utilisateur
  (accueil, séries, favoris, reprise, listes) sans rien supprimer ;
  le déblocage restaure immédiatement

### 5.3 Vidéos

- Recherchez une vidéo et modifiez titre/année/genres/synopsis
- Importez une affiche personnalisée
- **Télécharger** ouvre une boîte de dialogue pour enregistrer la vidéo en MKV
  ou MP4 avec la piste audio et les sous-titres intégrés/téléversés choisis
  (remux sans ré-encodage)
- **Récupérer** obtient les métadonnées de TMDB (nécessite `TMDB_API_KEY` et l’accès
  réseau à api.themoviedb.org)
- Les administrateurs peuvent choisir **TMDB ou un fournisseur personnalisé**
  et forcer l’écrasement des métadonnées existantes
- Les vidéos peuvent porter des **étiquettes** (manuelles ou issues d’un outil
  d’analyse IA optionnel) ; elles s’affichent sur la page détail et peuvent
  filtrer la recherche
- La page détail affiche des **vidéos similaires** ; la page de navigation
  propose un **nuage d’étiquettes** pour filtrer en un clic
- La page de navigation peut **enregistrer des filtres**, les réappliquer et
  créer des **collections intelligentes** nommées à partir du filtre courant
- La recherche propose un **tri par pertinence floue** qui tolère les fautes
  de frappe (titre / synopsis / nom de fichier)
- La liste vidéo admin supporte les **actions groupées** (étiqueter, effacer
  les étiquettes, mettre à la corbeille) et une **corbeille** avec restauration
  en un clic
- Les utilisateurs peuvent **commenter et noter** les vidéos (1-5 étoiles) ;
  l’accueil affiche un fil d’activité récente (commentaires et favoris)
- **Connexion unique** (OIDC) : la page de connexion propose un bouton SSO si
  le serveur est configuré avec un fournisseur d’identité
- **Contrôle parental** : les admins définissent une politique de
  classification par utilisateur et une classification par vidéo ; les
  utilisateurs peuvent verrouiller avec un PIN et déverrouiller le contenu
  classé pendant 5 minutes. Les bibliothèques peuvent aussi porter un **quota
  de stockage** vérifié lors des téléversements

### 5.4 Utilisateurs

- Changez les rôles (utilisateur/admin), réinitialisez les mots de passe, supprimez des comptes
- Garde-fous : impossible de supprimer son propre compte ; le dernier admin ne peut
  être ni supprimé ni rétrogradé

### 5.5 Contenu bloqué

- Bloquez tout média par titre (ex. un nom de série) sans supprimer fichiers ni enregistrements
- Les titres bloqués sont masqués pour tous les utilisateurs : accueil, séries, favoris,
  reprise de lecture et listes de lecture — le déblocage restaure immédiatement
- Utilisez la recherche pour prévisualiser quels médias un titre bloquerait avant de l’ajouter

### 5.6 Téléchargements (yt-dlp)

- Mettez en file toute URL de vidéo/playlist/chaîne ; choisissez un dossier
  cible, un format yt-dlp et un intervalle de répétition optionnel (heures) —
  le serveur télécharge via yt-dlp
- Progression en direct, annulation et nouvelle tentative après échec
- Les fichiers finis arrivent dans le dossier choisi et sont indexés
  automatiquement s’il fait partie d’une bibliothèque

## 6. Configuration

Tout se configure par variables d’environnement (table complète dans le README). Principales :

| Variable | Défaut | Rôle |
| --- | --- | --- |
| `PORT` | `8080` | Port d’écoute |
| `DATABASE_URL` | `postgres://localhost:5432/videocms` | Connexion base de données |
| `JWT_SECRET` | valeur dev | **Secret fort obligatoire en production** |
| `TMDB_API_KEY` | vide | Active le scraping de métadonnées |
| `SCAN_WORKERS` | `4` | Workers d’analyse parallèles |
| `WATCH_INTERVAL` | `30` | Secondes entre analyses incrémentales automatiques ; `0` désactive |
| `YTDLP_PATH` | `yt-dlp` dans le PATH | Binaire yt-dlp utilisé par la file de téléchargements |
| `HLS_HW_ACCEL` | vide (x264 logiciel) | Codec vidéo HLS : videotoolbox, nvenc, qsv ou vaapi ; vide = libx264 |
| `HLS_VAAPI_DEVICE` | `/dev/dri/renderD128` | Périphérique VAAPI (avec `HLS_HW_ACCEL=vaapi`) |
| `HLS_TONE_MAP` | `0` | `1` active le mappage de tons HDR→SDR dans le transcodage HLS |
| `SUBTITLE_OS_USERNAME` / `SUBTITLE_OS_PASSWORD` / `SUBTITLE_OS_API_KEY` | vide | Identifiants OpenSubtitles pour la recherche de sous-titres |
| `RTMP_INGEST_URL` | `rtmp://localhost:1935/live` | URL d’ingestion RTMP de base (nginx-rtmp ou équivalent) |
| `WHISPER_BIN` / `WHISPER_MODEL` | vide | CLI whisper.cpp et modèle pour la transcription vocale |
| `SCRAPE_CUSTOM_URL` | vide | Endpoint JSON de scraping personnalisé ; `%s` est remplacé par le titre encodé |
| `AI_TAG_BIN` | vide | Outil d’étiquetage IA externe (argument : chemin média, une étiquette par ligne) |
| `OIDC_ISSUER` / `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` / `OIDC_REDIRECT_URL` | vide | Paramètres de connexion unique OIDC |

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

**Comment masquer du contenu sans supprimer de fichiers ?**
Les administrateurs peuvent bloquer un titre (**Admin → Contenu bloqué**) ou une
bibliothèque entière (**Admin → Bibliothèques → Bloquer la bibliothèque**) ;
le contenu bloqué disparaît pour tous et partout, puis revient immédiatement au
déblocage. Les utilisateurs peuvent aussi masquer n’importe quel chemin serveur
pour eux-mêmes via le filtre de chemins.

## 8. Technologies et licence

Go + React + PostgreSQL, avec ffmpeg pour le traitement média. Interface entièrement
localisée (en/zh/fr/ja/de). Sous licence **Apache License 2.0**.

Pour les aspects internes, voir [l’architecture système](architecture.md).
