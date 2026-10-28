package main

import (
	"fmt"
	"net"
	"net/textproto"
	"regexp"
	"strings"
	"time"
)

// SMTPError es una respuesta del servidor que no fue 2xx/3xx.
type SMTPError struct {
	Code     int    // 421, 550...
	Enhanced string // "4.7.0", "5.1.1" (RFC 3463); vacío si el servidor no lo manda
	Msg      string
}

func (e *SMTPError) Error() string {
	return fmt.Sprintf("%d %s %s", e.Code, e.Enhanced, e.Msg)
}

var enhancedRe = regexp.MustCompile(`^([245]\.\d{1,3}\.\d{1,3})\s+`)

func parseReply(code int, msg string) *SMTPError {
	e := &SMTPError{Code: code, Msg: msg}
	if m := enhancedRe.FindStringSubmatch(msg); m != nil {
		e.Enhanced = m[1]
		e.Msg = strings.TrimSpace(msg[len(m[0]):])
	}
	return e
}

// deliver habla SMTP a mano (RFC 5321) para que se vea cada paso del diálogo.
// ponytail: sin STARTTLS ni AUTH; agregar STARTTLS antes de entregar a MX reales.
func deliver(addr, helo, from, to string, msg []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		// Sin conexión = temporal: el servidor puede volver.
		return &SMTPError{Code: 421, Enhanced: "4.4.1", Msg: err.Error()}
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(60 * time.Second))
	c := textproto.NewConn(conn)

	step := func(expect int, format string, args ...any) error {
		if format != "" {
			if err := c.PrintfLine(format, args...); err != nil {
				return &SMTPError{Code: 421, Enhanced: "4.4.2", Msg: err.Error()}
			}
		}
		code, msg, err := c.ReadResponse(expect)
		if err != nil {
			if code == 0 {
				return &SMTPError{Code: 421, Enhanced: "4.4.2", Msg: err.Error()}
			}
			return parseReply(code, msg)
		}
		return nil
	}

	if err := step(220, ""); err != nil { // saludo del servidor
		return err
	}
	if err := step(250, "EHLO %s", helo); err != nil {
		return err
	}
	if err := step(250, "MAIL FROM:<%s>", from); err != nil {
		return err
	}
	if err := step(250, "RCPT TO:<%s>", to); err != nil {
		return err
	}
	if err := step(354, "DATA"); err != nil {
		return err
	}
	w := c.DotWriter() // escapa líneas que empiezan con "." y cierra con "\r\n.\r\n"
	w.Write(msg)
	w.Close()
	if err := step(250, ""); err != nil {
		return err
	}
	c.PrintfLine("QUIT")
	return nil
}
