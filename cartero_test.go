package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		code     int
		enhanced string
		msg      string
		want     Verdict
	}{
		{550, "5.1.1", "user unknown", Hard},
		{421, "4.7.0", "try again later", Reputation},
		{550, "5.7.1", "message rejected", Reputation},
		{554, "", "blocked using spamhaus", Reputation},
		{451, "4.3.0", "temporary failure", Temporary},
		{421, "", "too many connections", Temporary},
	}
	for _, c := range cases {
		if got := classify(&SMTPError{c.code, c.enhanced, c.msg}); got != c.want {
			t.Errorf("%d %s %q: got %s, want %s", c.code, c.enhanced, c.msg, got, c.want)
		}
	}
	if classify(nil) != Delivered {
		t.Error("nil debe ser entregado")
	}
}

func TestParseReply(t *testing.T) {
	e := parseReply(550, "5.1.1 The email account does not exist")
	if e.Enhanced != "5.1.1" || e.Msg != "The email account does not exist" {
		t.Errorf("got %+v", e)
	}
}

// fakeServer imita un MX: rechaza RCPT con `firstRcpt` las primeras `fails` veces y después acepta.
func fakeServer(t *testing.T, fails int, firstRcpt string) string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for n := 0; ; n++ {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(n int) {
				defer conn.Close()
				r := bufio.NewReader(conn)
				fmt.Fprint(conn, "220 fake ESMTP\r\n")
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(cmd, "RCPT") && n < fails:
						fmt.Fprintf(conn, "%s\r\n", firstRcpt)
					case cmd == "DATA":
						fmt.Fprint(conn, "354 go ahead\r\n")
						for {
							l, _ := r.ReadString('\n')
							if l == ".\r\n" || l == "" {
								break
							}
						}
						fmt.Fprint(conn, "250 2.0.0 queued\r\n")
					case cmd == "QUIT":
						fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						fmt.Fprint(conn, "250 ok\r\n")
					}
				}
			}(n)
		}
	}()
	return ln.Addr().String()
}

func TestSendRetriesTemporary(t *testing.T) {
	addr := fakeServer(t, 2, "451 4.3.0 temporary failure")
	v, err := send([]string{addr}, "test", "a@x.test", "b@y.test", []byte("hola\r\n"), 4, time.Millisecond)
	if v != Delivered || err != nil {
		t.Fatalf("debió entregar al 3er intento: %s %v", v, err)
	}
}

func TestSendStopsOnHard(t *testing.T) {
	addr := fakeServer(t, 99, "550 5.1.1 user unknown")
	v, _ := send([]string{addr}, "test", "a@x.test", "b@y.test", []byte("hola\r\n"), 4, time.Millisecond)
	if v != Hard {
		t.Fatalf("got %s, want duro", v)
	}
}
