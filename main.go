package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"

	"ws-codingame-insalgo/internal/realtime"
)

// TODO: Envoie du lien quand un client se connecte ; Déploiement du site ; Lien avec le bot discord pour pouvoir maj depuis le discord

// securityHeaders ajoute des en-têtes de sécurité standard.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		// CSP très permissif pour ce POC; à durcir selon besoins
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' wss:; style-src 'self' 'unsafe-inline'; script-src 'self'")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		// HSTS seulement en HTTPS (voir plus bas)
		next.ServeHTTP(w, r)
	})
}

// allowedHostsMiddleware limite les hosts servis si ALLOWED_HOSTS est défini (liste séparée par virgules).
func allowedHostsMiddleware(next http.Handler) http.Handler {
	allowed := strings.Split(strings.TrimSpace(os.Getenv("ALLOWED_HOSTS")), ",")
	set := make(map[string]struct{})
	for _, h := range allowed {
		h = strings.TrimSpace(h)
		if h != "" {
			set[strings.ToLower(h)] = struct{}{}
		}
	}
	if len(set) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.ToLower(r.Host)
		if _, ok := set[host]; !ok {
			http.Error(w, "Host non autorisé", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Hub WebSocket
	hub := realtime.NewHub()

	// Origines autorisées pour WS via env ALLOWED_ORIGINS (schéma+host+port)
	var allowedOrigins []string
	if v := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS")); v != "" {
		for _, item := range strings.Split(v, ",") {
			item = strings.TrimSpace(item)
			if item != "" {
				allowedOrigins = append(allowedOrigins, item)
			}
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.Handler(allowedOrigins))
	mux.Handle("/", http.FileServer(http.Dir("static")))

	// Wrap middlewares
	handler := securityHeaders(allowedHostsMiddleware(mux))

	// Lecture du terminal et diffusion
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Println("Tapez vos messages ci-dessous et appuyez sur Entrée pour les envoyer aux clients...")
		for scanner.Scan() {
			line := scanner.Text()
			hub.Broadcast(realtime.Message{Content: line})
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Erreur de lecture depuis le terminal: %v", err)
		}
	}()

	// Choisir HTTP ou HTTPS
	addrHTTP := ":8080"
	addrHTTPS := ":8443" // par défaut local; 443 en prod

	certFile := strings.TrimSpace(os.Getenv("TLS_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv("TLS_KEY_FILE"))
	domain := strings.TrimSpace(os.Getenv("DOMAIN"))

	srv := &http.Server{
		Addr:    addrHTTP,
		Handler: handler,
	}

	// Arrêt gracieux
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-shutdownCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	// Si certs fournis, servir en HTTPS
	if certFile != "" && keyFile != "" {
		srv.Addr = addrHTTPS
		// HSTS en HTTPS
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
			securityHeaders(allowedHostsMiddleware(mux)).ServeHTTP(w, r)
		})
		srv.Handler = handler
		log.Printf("Serveur démarré en HTTPS sur https://localhost%s", addrHTTPS)
		log.Printf("WebSocket disponible sur wss://localhost%s/ws", addrHTTPS)
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erreur serveur HTTPS: %v", err)
		}
		return
	}

	// Sinon si un domaine est fourni, tenter autocert (Let’s Encrypt)
	if domain != "" {
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(domain),
			Cache:      autocert.DirCache(".cert-cache"),
		}
		// HTTP-01 challenge sur :80
		go func() {
			httpSrv := &http.Server{Addr: ":80", Handler: m.HTTPHandler(nil)}
			log.Printf("Serveur challenge HTTP démarré sur :80 pour %s", domain)
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("Erreur challenge HTTP: %v", err)
			}
		}()
		// Serveur HTTPS principal sur :443
		httpsSrv := &http.Server{
			Addr:      ":443",
			Handler:   handler,
			TLSConfig: &tls.Config{GetCertificate: m.GetCertificate},
		}
		log.Printf("Serveur démarré en HTTPS sur https://%s", domain)
		log.Printf("WebSocket disponible sur wss://%s/ws", domain)
		if err := httpsSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erreur serveur HTTPS (autocert): %v", err)
		}
		return
	}

	// Fallback: HTTP
	log.Printf("Serveur démarré sur http://localhost%s", addrHTTP)
	log.Printf("WebSocket disponible sur ws://localhost%s/ws", addrHTTP)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Erreur serveur HTTP: %v", err)
	}
}
