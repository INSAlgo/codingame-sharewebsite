package realtime

import (
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

// Message représente la charge utile envoyée aux clients.
type Message struct {
	Content string `json:"content"`
}

// Hub gère l'ensemble des clients connectés et la diffusion des messages.
// Il mémorise également le dernier message diffusé pour l'envoyer aux nouveaux clients.
type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]struct{}

	lastMsg Message
	lastSet bool
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]struct{})}
}

// Broadcast envoie un message JSON à tous les clients et mémorise ce message
// comme étant le dernier diffusé.
func (h *Hub) Broadcast(msg Message) {
	// Snapshot de la liste des clients et mise à jour du dernier message,
	// sans faire d'I/O sous verrou.
	h.mu.Lock()
	h.lastMsg = msg
	h.lastSet = true

	clients := make([]*websocket.Conn, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		if err := c.WriteJSON(msg); err != nil {
			log.Printf("write error to %s: %v", c.RemoteAddr(), err)
		}
	}
}

// add enregistre un client.
func (h *Hub) add(c *websocket.Conn) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

// remove supprime un client.
func (h *Hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// Handler retourne un http.HandlerFunc pour /ws avec vérification d'origine stricte.
// allowedOrigins: liste d'origines autorisées (schéma+host+port). Si vide, seule l'origine exacte du host courant est acceptée.
func (h *Hub) Handler(allowedOrigins []string) http.HandlerFunc {
	// normaliser les origines autorisées pour comparaison insensible à la casse
	norm := func(in []string) map[string]struct{} {
		m := make(map[string]struct{}, len(in))
		for _, v := range in {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			m[strings.ToLower(v)] = struct{}{}
		}
		return m
	}(allowedOrigins)

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := strings.ToLower(r.Header.Get("Origin"))
			if origin == "" {
				// Certains navigateurs n'envoient pas Origin pour ws depuis la même origine
				return true
			}
			if len(norm) == 0 {
				// Autoriser même origine uniquement
				// Construire l'URL d'origine attendue depuis la requête
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				expected := scheme + "://" + r.Host
				return origin == strings.ToLower(expected)
			}
			_, ok := norm[origin]
			return ok
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade error: %v", err)
			return
		}
		h.add(conn)
		log.Printf("Client connecté: %s", conn.RemoteAddr())

		// À la connexion, envoyer le dernier message s'il existe
		var (
			hasLast bool
			last    Message
		)
		h.mu.RLock()
		hasLast = h.lastSet
		if hasLast {
			last = h.lastMsg
		}
		h.mu.RUnlock()
		if hasLast {
			if err := conn.WriteJSON(last); err != nil {
				log.Printf("erreur envoi dernier message à %s: %v", conn.RemoteAddr(), err)
			}
		}

		go func(c *websocket.Conn) {
			// Lire jusqu'à fermeture pour détecter déconnexion
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					break
				}
			}
			h.remove(c)
			_ = c.Close()
			log.Printf("Client déconnecté: %s", c.RemoteAddr())
		}(conn)
	}
}
