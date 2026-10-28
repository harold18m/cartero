package main

import (
	"net"
	"strings"
	"sync"
	"time"
)

// provider agrupa los MX por dueño: gmail.com y cualquier dominio con Google
// Workspace comparten límites porque llegan a la misma infraestructura.
func provider(mxAddr string) string {
	host, _, err := net.SplitHostPort(mxAddr)
	if err != nil {
		host = mxAddr
	}
	host = strings.ToLower(host)
	switch {
	case strings.HasSuffix(host, ".google.com"), strings.HasSuffix(host, ".googlemail.com"):
		return "gmail"
	case strings.HasSuffix(host, ".outlook.com"):
		return "outlook"
	case strings.HasSuffix(host, ".yahoodns.net"):
		return "yahoo"
	}
	return "otros"
}

// Intervalo mínimo entre conexiones por proveedor.
// ponytail: valores fijos y conservadores, no publicados por los proveedores;
// ajustarlos según cuántos 4.7.x devuelva cada uno.
var intervals = map[string]time.Duration{
	"gmail":   100 * time.Millisecond, // 10/s
	"outlook": 200 * time.Millisecond, // 5/s, el más estricto con remitentes nuevos
	"yahoo":   500 * time.Millisecond, // 2/s
	"otros":   50 * time.Millisecond,  // 20/s
}

// limiter reparte turnos espaciados `every`; cada wait reserva el siguiente.
// El ritmo se adapta como TCP (AIMD): frena fuerte cuando el proveedor se queja
// y acelera de a poco cuando acepta, sin pasar nunca del ritmo base.
type limiter struct {
	mu    sync.Mutex
	next  time.Time
	base  time.Duration
	every time.Duration
}

const maxInterval = 30 * time.Second

// slowDown duplica el intervalo: la mitad de envíos por segundo.
func (l *limiter) slowDown() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.every = min(l.every*2, maxInterval)
}

// speedUp acorta el intervalo un 10% hasta volver al base.
func (l *limiter) speedUp() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.every = max(l.every*9/10, l.base)
}

func (l *limiter) wait() {
	l.mu.Lock()
	now := time.Now()
	if l.next.Before(now) {
		l.next = now
	}
	turn := l.next
	l.next = l.next.Add(l.every)
	l.mu.Unlock()
	time.Sleep(time.Until(turn))
}

var (
	limitersMu sync.Mutex
	limiters   = map[string]*limiter{}
)

func limiterFor(mxAddr string) *limiter {
	p := provider(mxAddr)
	limitersMu.Lock()
	defer limitersMu.Unlock()
	if limiters[p] == nil {
		limiters[p] = &limiter{base: intervals[p], every: intervals[p]}
	}
	return limiters[p]
}

func (l *limiter) interval() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.every
}
