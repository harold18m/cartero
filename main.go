package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// mxHosts devuelve los MX del dominio en orden de preferencia (RFC 5321 §5.1).
func mxHosts(domain string) []string {
	mx, err := net.LookupMX(domain)
	if err != nil || len(mx) == 0 {
		return []string{domain + ":25"} // sin MX se usa el registro A del dominio
	}
	sort.Slice(mx, func(i, j int) bool { return mx[i].Pref < mx[j].Pref })
	hosts := make([]string, len(mx))
	for i, m := range mx {
		hosts[i] = strings.TrimSuffix(m.Host, ".") + ":25"
	}
	return hosts
}

// send reintenta solo lo temporal, con backoff exponencial.
// ponytail: cola en memoria y sleep bloqueante; pasar a cola persistente con
// próximo intento programado cuando haya más de un mensaje en vuelo.
func send(hosts []string, helo, from, to string, msg []byte, attempts int, base time.Duration) (Verdict, error) {
	var err error
	for i := 0; i < attempts; i++ {
		for _, h := range hosts {
			limiterFor(h).wait()
			err = deliver(h, helo, from, to, msg)
			v := classify(err)
			log.Printf("intento %d via %s -> %s %v", i+1, h, v, err)
			if v != Temporary {
				return v, err
			}
		}
		if i < attempts-1 {
			time.Sleep(base << i) // 1x, 2x, 4x...
		}
	}
	return Temporary, err
}

func main() {
	from := flag.String("from", "cartero@localhost", "remitente")
	to := flag.String("to", "", "destinatarios separados por coma")
	relay := flag.String("relay", "localhost:1025", "host:puerto fijo (Mailpit); vacío = resolver MX")
	flag.Parse()
	if *to == "" {
		log.Fatal("falta -to")
	}

	// Todos en paralelo: el límite por proveedor es lo único que los espacia.
	var wg sync.WaitGroup
	var failed atomic.Bool
	for _, rcpt := range strings.Split(*to, ",") {
		rcpt := strings.TrimSpace(rcpt)
		hosts := []string{*relay}
		if *relay == "" {
			hosts = mxHosts(rcpt[strings.LastIndex(rcpt, "@")+1:])
		}
		msg := fmt.Sprintf("From: <%s>\r\nTo: <%s>\r\nSubject: Hola desde cartero\r\nDate: %s\r\n\r\nPrimer correo de cartero.\r\n",
			*from, rcpt, time.Now().Format(time.RFC1123Z))
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := send(hosts, "localhost", *from, rcpt, []byte(msg), 4, time.Second)
			fmt.Printf("%s: %s\n", rcpt, v)
			if err != nil {
				failed.Store(true)
			}
		}()
	}
	wg.Wait()
	if failed.Load() {
		os.Exit(1)
	}
}
