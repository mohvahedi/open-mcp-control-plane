package server

import (
	"sync"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
)

// mcpSession tracks a Streamable HTTP / SSE client session.
type mcpSession struct {
	ID        string
	ClientID  string
	ProfileID string
	CreatedAt time.Time
	LastSeen  time.Time
	// events is a fan-out channel for SSE subscribers (non-blocking sends drop if full).
	subs map[chan []byte]struct{}
	mu   sync.Mutex
}

type sessionHub struct {
	mu       sync.Mutex
	sessions map[string]*mcpSession
	ttl      time.Duration
}

func newSessionHub(ttl time.Duration) *sessionHub {
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	h := &sessionHub{sessions: map[string]*mcpSession{}, ttl: ttl}
	go h.reaper()
	return h
}

func (h *sessionHub) reaper() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for range t.C {
		h.mu.Lock()
		now := time.Now()
		for id, s := range h.sessions {
			if now.Sub(s.LastSeen) > h.ttl {
				s.closeSubs()
				delete(h.sessions, id)
			}
		}
		h.mu.Unlock()
	}
}

func (h *sessionHub) create(client domain.Client, profile domain.Profile) *mcpSession {
	id := newID("sess")
	now := time.Now().UTC()
	s := &mcpSession{
		ID: id, ClientID: client.ID, ProfileID: profile.ID,
		CreatedAt: now, LastSeen: now, subs: map[chan []byte]struct{}{},
	}
	h.mu.Lock()
	h.sessions[id] = s
	h.mu.Unlock()
	return s
}

func (h *sessionHub) get(id string) (*mcpSession, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.sessions[id]
	if !ok {
		return nil, false
	}
	s.LastSeen = time.Now().UTC()
	return s, true
}

func (h *sessionHub) delete(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.sessions[id]
	if !ok {
		return false
	}
	s.closeSubs()
	delete(h.sessions, id)
	return true
}

func (h *sessionHub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.sessions)
}

func (h *sessionHub) list() []map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]map[string]any, 0, len(h.sessions))
	for _, s := range h.sessions {
		out = append(out, map[string]any{
			"id": s.ID, "client_id": s.ClientID, "profile_id": s.ProfileID,
			"created_at": s.CreatedAt, "last_seen": s.LastSeen,
		})
	}
	return out
}

func (s *mcpSession) subscribe() chan []byte {
	ch := make(chan []byte, 16)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch
}

func (s *mcpSession) unsubscribe(ch chan []byte) {
	s.mu.Lock()
	_, ok := s.subs[ch]
	if ok {
		delete(s.subs, ch)
	}
	s.mu.Unlock()
	if ok {
		close(ch)
	}
}

func (s *mcpSession) publish(event []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastSeen = time.Now().UTC()
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
			// drop if slow consumer
		}
	}
}

func (s *mcpSession) closeSubs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subs {
		close(ch)
		delete(s.subs, ch)
	}
}
