package main

import "strings"

type Verdict string

const (
	Delivered  Verdict = "entregado"
	Temporary  Verdict = "temporal"   // reintentar con backoff
	Hard       Verdict = "duro"       // no reintentar; suprimir la dirección
	Reputation Verdict = "reputacion" // no es culpa del destinatario: bajar ritmo o cambiar IP
)

// classify decide qué hacer con una respuesta. El código extendido manda sobre el
// básico porque es más preciso (un 550 puede ser "no existe" o "te bloqueé por spam").
func classify(err error) Verdict {
	if err == nil {
		return Delivered
	}
	e, ok := err.(*SMTPError)
	if !ok {
		return Temporary
	}
	text := strings.ToLower(e.Msg)
	// x.7.x = política/seguridad. Si además habla de reputación o spam, es contra nosotros.
	if strings.HasPrefix(e.Enhanced, "5.7.") || strings.HasPrefix(e.Enhanced, "4.7.") ||
		strings.Contains(text, "spam") || strings.Contains(text, "reputation") || strings.Contains(text, "blocked") {
		return Reputation
	}
	if strings.HasPrefix(e.Enhanced, "4.") || (e.Enhanced == "" && e.Code/100 == 4) {
		return Temporary
	}
	return Hard
}
