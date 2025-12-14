# Build l'image
```bash
docker build -t codingame-sharewebsite .
```

# Run avec autocert (Let's Encrypt automatique)
```bash
docker run -d \
  -p 80:80 \
  -p 443:443 \
  -e DOMAIN=codingame-share.insalgo.fr \
  -e ALLOWED_HOSTS=codingame-share.insalgo.fr \
  -e ALLOWED_ORIGINS=https://codingame-share.insalgo.fr \
  -e BROADCAST_API_KEY=votre-cle-secrete-ici \
  -v $(pwd)/cert-cache:/app/.cert-cache \
  --restart unless-stopped \
  --name codingame \
  codingame-sharewebsite

# Voir les logs
docker logs -f codingame
```

**Prérequis sur le VPS :**
- Ports 80 et 443 ouverts dans le firewall
- DNS `codingame-share.insalgo.fr` pointe vers l'IP du VPS
- Aucun autre service sur les ports 80/443 (arrêtez Nginx s'il tourne)

**Pour tester en local (sans HTTPS) :**
```bash
docker build -t codingame-sharewebsite . && docker run --rm -p 8080:8080
```
