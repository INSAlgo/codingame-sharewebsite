# codingame-sharewebsite — Guide de déploiement (OVH VPS + Nginx)

Ce document décrit pas à pas comment déployer l’application sur un VPS OVH en production, derrière un reverse-proxy Nginx, avec certificat TLS Let’s Encrypt et gestion de service via systemd. Cette documentation couvre spécifiquement l’Option A: terminaison TLS côté Nginx (Certbot) et application Go en HTTP interne (port :8080).

L’application est un serveur Go qui:
- Sert les fichiers statiques depuis le dossier `static`
- Expose un endpoint WebSocket sur `/ws`
- Diffuse aux clients des messages saisis via stdin
- Mémorise et envoie le dernier message à chaque nouvelle connexion (si vous avez pris la modification correspondante)

L’application inclut un endpoint HTTP de broadcast sécurisé (/api/broadcast) utilisable via curl et par des bots (clé requise).

Note (Option A - Terminaison TLS côté Nginx): l’application Go tourne en HTTP interne (:8080) et Nginx gère HTTPS/WSS. Définissez impérativement dans le service systemd:
- ALLOWED_HOSTS=codingame-share.insalgo.fr
- ALLOWED_ORIGINS=https://codingame-share.insalgo.fr

Nginx transmettra X-Forwarded-Proto=https, et le serveur Go honorera cet en-tête pour valider correctement l’origine WebSocket derrière le reverse-proxy.

----------------------------------------------------------------

## Sommaire

1. Prérequis
2. DNS OVH (sous-domaine)
3. Préparation du VPS
4. Installation des dépendances (Go, Nginx, Certbot)
5. Déploiement des sources et build
6. Service systemd
7. Configuration Nginx
8. Certificat TLS avec Certbot
9. Variables d’environnement (ALLOWED_HOSTS, ALLOWED_ORIGINS, etc.)
10. Tests
11. Logs et opérations courantes
12. Mises à jour et redeploy
13. Sécurité et bonnes pratiques
14. Endpoint HTTP de broadcast sécurisé (/api/broadcast)
15. Dépannage (FAQ)

----------------------------------------------------------------

## 1) Prérequis

- Un VPS OVH (Ubuntu/Debian).
- Un domaine géré chez OVH (ex.: `insalgo.fr`).
- Un sous-domaine dédié au service (ex.: `codingame-share.insalgo.fr`).
- Accès SSH au VPS avec un utilisateur non-root disposant de sudo.
- Ports 80 et 443 ouverts (firewall VPS et éventuellement firewall réseau OVH).

Remarque Go: le fichier `go.mod` indique `go 1.25.0`. Selon votre distribution, cette version peut ne pas être disponible. Une version récente (par ex. Go 1.22.x) fonctionne très bien. Si la compilation échoue à cause de la directive, vous pouvez la remplacer par `go 1.22` dans `go.mod` sans changer les dépendances.

----------------------------------------------------------------

## 2) DNS OVH (sous-domaine)

Dans l’espace client OVH:
- Rendez-vous dans Noms de domaine → `insalgo.fr` → Zone DNS.
- Ajoutez:
  - Un enregistrement A pour `codingame-share` vers l’IPv4 de votre VPS.
  - Un enregistrement AAAA (optionnel) vers l’IPv6 du VPS si disponible.
- Attendez la propagation (souvent rapide).
- Vérifiez:
  - `dig +short A codingame-share.insalgo.fr`
  - `dig +short AAAA codingame-share.insalgo.fr`

----------------------------------------------------------------

## 3) Préparation du VPS

- Mettez le système à jour.
- Créez un utilisateur applicatif (ex.: `app`) et configurez l’accès SSH par clé.
- Activez et configurez le firewall UFW:
  - Autorisez 22/tcp, 80/tcp et 443/tcp.

Exemple (exécuter en root, adaptez):
    apt update && apt -y upgrade
    adduser app
    usermod -aG sudo app
    apt -y install ufw
    ufw default deny incoming
    ufw default allow outgoing
    ufw allow 22/tcp
    ufw allow 80/tcp
    ufw allow 443/tcp
    ufw --force enable
    ufw status verbose

----------------------------------------------------------------

## 4) Installation des dépendances

Installez Nginx et Certbot:
    apt -y install nginx certbot python3-certbot-nginx

Installez Go (méthode officielle simplifiée):
    cd /tmp
    curl -fsSL -o go.tgz https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
    rm -rf /usr/local/go
    tar -C /usr/local -xzf go.tgz
    echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
    chmod +x /etc/profile.d/go.sh
    source /etc/profile.d/go.sh
    go version

Note: si vous préférez utiliser le paquet Go de la distribution, adaptez les chemins et versions.

----------------------------------------------------------------

## 5) Déploiement des sources et build

- Placez le projet dans `/opt/codingame-sharewebsite`.
- Construisez le binaire `server`.

Exemple:
    mkdir -p /opt/codingame-sharewebsite
    # Copiez les fichiers du projet dans /opt/codingame-sharewebsite (via git clone, scp, rsync…)
    cd /opt/codingame-sharewebsite
    go mod download
    go build -o /opt/codingame-sharewebsite/server ./main.go

Assurez les permissions:
    chown -R app:app /opt/codingame-sharewebsite

Important: l’application sert `static` avec un chemin relatif. Le `WorkingDirectory` du service doit pointer sur `/opt/codingame-sharewebsite`.

----------------------------------------------------------------

## 6) Service systemd

Un exemple de service est fourni:
- `deploy/codingame-sharewebsite.service`

Copiez-le dans:
    /etc/systemd/system/codingame-sharewebsite.service

Champs à ajuster dans ce fichier:
- `User` / `Group`: l’utilisateur non-root qui exécute l’app (ex.: `app`).
- `WorkingDirectory`: dossier qui contient le binaire `server` et `static` (par défaut `/opt/codingame-sharewebsite`).
- `ExecStart`: chemin absolu du binaire (par défaut `/opt/codingame-sharewebsite/server`).
- `Environment`:
  - `ALLOWED_HOSTS`: `codingame-share.insalgo.fr`
  - `ALLOWED_ORIGINS`: `https://codingame-share.insalgo.fr`
  - `BROADCAST_API_KEY`: optionnel, seulement si vous implémentez un endpoint d’API interne.

Activez et démarrez:
    systemctl daemon-reload
    systemctl enable --now codingame-sharewebsite
    journalctl -u codingame-sharewebsite -f

----------------------------------------------------------------

## 7) Configuration Nginx

Un vhost d’exemple est fourni:
- `deploy/nginx-codingame-share.conf`

Installez-le:
    cp deploy/nginx-codingame-share.conf /etc/nginx/sites-available/codingame-share

Champs à ajuster:
- `server_name`: `codingame-share.insalgo.fr`
- `ssl_certificate` et `ssl_certificate_key`:
  - Si vous utilisez Certbot avec `--nginx`, ils seront gérés automatiquement.
  - Sinon, définissez les chemins (Let’s Encrypt: `/etc/letsencrypt/live/codingame-share.insalgo.fr/...`).
- `proxy_pass`: l’URL interne vers l’application (par défaut `http://127.0.0.1:8080`).

Activez le site:
    ln -s /etc/nginx/sites-available/codingame-share /etc/nginx/sites-enabled/codingame-share
    nginx -t
    systemctl reload nginx

----------------------------------------------------------------

## 8) Certificat TLS avec Certbot

Générez et installez le certificat:
    certbot --nginx -d codingame-share.insalgo.fr --redirect

- Certbot ajoute le bloc HTTPS, remplit les chemins `ssl_certificate` et `ssl_certificate_key`, et force la redirection HTTP→HTTPS.
- Renouvellement automatique géré par Certbot (vérifiable via `systemctl list-timers | grep certbot`).

----------------------------------------------------------------

## 9) Variables d’environnement indispensables

Dans le service systemd:
- `ALLOWED_HOSTS`: liste d’hôtes autorisés (sans schéma). Exemple:
  - `ALLOWED_HOSTS=codingame-share.insalgo.fr`
- `ALLOWED_ORIGINS`: schéma + host (+ port si non-443) autorisés pour l’upgrade WebSocket. Derrière Nginx en HTTPS:
  - `ALLOWED_ORIGINS=https://codingame-share.insalgo.fr`

Pourquoi `ALLOWED_ORIGINS` est crucial:
- Le code WS vérifie l’en-tête `Origin`. Comme Nginx termine TLS, côté app `r.TLS` est généralement `nil`, donc la logique de "même origine" ne suffit pas. Définir explicitement l’origine HTTPS évite les refus de connexion WS.

Optionnel:
- `BROADCAST_API_KEY`: si vous implémentez un endpoint HTTP interne `/api/broadcast`, définissez une clé secrète ici et sécurisez l’API côté serveur.

----------------------------------------------------------------

## 10) Tests

- Redirection HTTP→HTTPS:
    curl -I http://codingame-share.insalgo.fr
- Accès HTTPS:
    curl -I https://codingame-share.insalgo.fr
- WebSocket (depuis le navigateur):
  - Ouvrez la console réseau, vérifiez la connexion à `wss://codingame-share.insalgo.fr/ws`.

----------------------------------------------------------------

## 11) Logs et opérations courantes

- Logs de l’application:
    journalctl -u codingame-sharewebsite -f
- Redémarrer l’application:
    systemctl restart codingame-sharewebsite
- Vérifier Nginx:
    nginx -t
    systemctl reload nginx
- Logs Nginx:
    tail -f /var/log/nginx/access.log /var/log/nginx/error.log

----------------------------------------------------------------

## 12) Mises à jour et redeploy

- Copier les changements sur le serveur (git pull, rsync, scp).
- Rebuilder:
    cd /opt/codingame-sharewebsite
    go build -o /opt/codingame-sharewebsite/server ./main.go
- Redémarrer:
    systemctl restart codingame-sharewebsite
- Vérifier:
    journalctl -u codingame-sharewebsite -f

Astuce: pour minimiser l’indisponibilité, build d’abord, puis faites un `systemctl restart` (quelques millisecondes de coupure WS sont attendues).

----------------------------------------------------------------

## 13) Sécurité et bonnes pratiques

- Service non-root: exécutez via un utilisateur dédié (ex.: `app`).
- Firewall: autorisez uniquement 22, 80, 443. Rien d’autre en entrée.
- Nginx:
  - Activer HTTP/2.
  - Utiliser des en-têtes de sécurité (HSTS, X-Content-Type-Options, X-Frame-Options).
- Application:
  - `ALLOWED_HOSTS` restreint l’hôte servi.
  - `ALLOWED_ORIGINS` restreint l’origine WS.
- Secrets:
  - Ne jamais commiter `BROADCAST_API_KEY`.
  - Placez les secrets dans les variables d’environnement du service systemd ou dans un fichier d’environnement protégé.

----------------------------------------------------------------

## 14) Endpoint HTTP de broadcast sécurisé (/api/broadcast)

L’application inclut un endpoint POST `/api/broadcast` pour diffuser un message à tous les clients connectés. Il est protégé par une clé à définir dans la variable d’environnement `BROADCAST_API_KEY` du service systemd.

Authentification:
- En-tête `Authorization: Bearer <votre_clé>` ou `X-Api-Key: <votre_clé>`

Corps de la requête:
- JSON: `{"content":"votre message"}`
- Texte brut: corps = message

Exemples curl:
- JSON:
    curl -X POST "https://codingame-share.insalgo.fr/api/broadcast" \
      -H "Authorization: Bearer VOTRE_CLE_SECRETE" \
      -H "Content-Type: application/json" \
      -d '{"content":"Hello depuis curl"}'
- Texte brut:
    curl -X POST "https://codingame-share.insalgo.fr/api/broadcast" \
      -H "Authorization: Bearer VOTRE_CLE_SECRETE" \
      --data 'Hello en texte'

Configuration côté service:
- Dans `/etc/systemd/system/codingame-sharewebsite.service`, ajoutez:
    Environment=BROADCAST_API_KEY=change_me
- Puis rechargez et redémarrez:
    systemctl daemon-reload
    systemctl restart codingame-sharewebsite

Nginx:
- Aucune configuration spéciale n’est requise: le bloc `location /` proxy déjà l’API vers l’application.
- Assurez-vous que `proxy_set_header X-Forwarded-Proto https;` est présent (déjà inclus dans l’exemple fourni).

Réponses de l’API:
- 204 No Content: diffusion réussie
- 401 Non autorisé: clé absente/incorrecte
- 400 Requête invalide: JSON invalide ou `content` vide

Sécurité:
- Utilisez une clé longue et aléatoire.
- Ne stockez pas la clé en clair dans des dépôts publics ou scripts versionnés.

----------------------------------------------------------------

## 15) Dépannage (FAQ)

- Le WebSocket refuse la connexion (erreur d’origine).
  - Vérifiez `ALLOWED_ORIGINS=https://codingame-share.insalgo.fr` dans le service systemd.
  - Redémarrez le service si vous avez modifié les variables d’environnement.

- La page charge mais le WS ne se connecte pas en production.
  - Vérifiez la configuration Nginx du bloc `/ws`:
    - `proxy_set_header Upgrade $http_upgrade;`
    - `proxy_set_header Connection "upgrade";`
    - `proxy_http_version 1.1;`
  - Vérifiez que vous utilisez `wss://` côté client (ce que fait le code de la page si elle est servie en HTTPS).

- Certificat Let’s Encrypt non obtenu.
  - Vérifiez que `codingame-share.insalgo.fr` pointe bien vers l’IP du VPS.
  - Assurez-vous que le port 80 est ouvert depuis Internet.
  - Regardez `/var/log/nginx/error.log` et `journalctl -u nginx`.

- L’app ne trouve pas le dossier `static`.
  - Vérifiez que `WorkingDirectory` du service systemd est `/opt/codingame-sharewebsite`.
  - Vérifiez que le dossier `static` est présent à cet emplacement.

- Le build échoue à cause de la version Go.
  - Essayez avec Go 1.22.x.
  - Si nécessaire, remplacez la directive `go 1.25.0` par `go 1.22` dans `go.mod`.

----------------------------------------------------------------

## Annexes

Fichiers fournis dans ce repo:
- Service systemd d’exemple:
  - `deploy/codingame-sharewebsite.service`
- Vhost Nginx d’exemple:
  - `deploy/nginx-codingame-share.conf`

Récapitulatif des étapes rapides:
1. DNS: `codingame-share.insalgo.fr` → IP du VPS
2. VPS: créer utilisateur, UFW, installer Nginx/Certbot/Go
3. Déployer sources dans `/opt/codingame-sharewebsite`, build `server`
4. Installer et ajuster le service systemd, définir `ALLOWED_HOSTS` et `ALLOWED_ORIGINS`
5. Activer et démarrer le service
6. Installer le vhost Nginx, activer, recharger
7. `certbot --nginx -d codingame-share.insalgo.fr --redirect`
8. Tester page et WebSocket

Bon déploiement.