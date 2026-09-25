package auth

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
)

type attemptWindow struct {
	count int
	reset time.Time
}

type limiter struct {
	mu      sync.Mutex
	windows map[string]attemptWindow
	limit   int
	window  time.Duration
}

func newLimiter(limit int, window time.Duration) *limiter {
	return &limiter{windows: make(map[string]attemptWindow), limit: limit, window: window}
}

func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.windows) > 4096 {
		for candidate, entry := range l.windows {
			if entry.reset.Before(now) {
				delete(l.windows, candidate)
			}
		}
	}
	entry := l.windows[key]
	if entry.reset.Before(now) {
		entry = attemptWindow{reset: now.Add(l.window)}
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	l.windows[key] = entry
	return true
}

func (l *limiter) clear(key string) {
	l.mu.Lock()
	delete(l.windows, key)
	l.mu.Unlock()
}

func passwordPolicy(password, username, email string) string {
	if len(password) < 12 || len(password) > 128 {
		return "Le mot de passe doit contenir entre 12 et 128 caractères."
	}
	var lower, upper, digit, symbol bool
	for _, char := range password {
		lower = lower || unicode.IsLower(char)
		upper = upper || unicode.IsUpper(char)
		digit = digit || unicode.IsDigit(char)
		symbol = symbol || (!unicode.IsLetter(char) && !unicode.IsDigit(char) && !unicode.IsSpace(char))
	}
	if !lower || !upper || !digit || !symbol {
		return "Utilisez une minuscule, une majuscule, un chiffre et un symbole."
	}
	normalized := strings.ToLower(password)
	emailName := strings.Split(strings.ToLower(email), "@")[0]
	if (len(username) >= 3 && strings.Contains(normalized, strings.ToLower(username))) || (len(emailName) >= 3 && strings.Contains(normalized, emailName)) {
		return "Le mot de passe ne doit pas contenir votre pseudo ou votre adresse e-mail."
	}
	return ""
}

func requestIP(r *http.Request) string {
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(forwarded) - 1; i >= 0; i-- {
		if value := strings.TrimSpace(forwarded[i]); net.ParseIP(value) != nil {
			return value
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
