# cartero

Un MTA mínimo en Go para aprender cómo se entrega un correo: el diálogo SMTP,
los reintentos y qué significa cada rebote.

## Probar

```sh
mailpit                                  # destino falso: SMTP en :1025, web en http://localhost:8025
go run . -to prueba@ejemplo.test         # entrega a Mailpit
go test ./...                            # incluye un servidor SMTP falso que responde 451 y 550
```

`-relay ""` resuelve el MX real del destinatario. Casi todos los proveedores bloquean
el puerto 25 de salida, así que practica contra Mailpit.

## Qué hace hoy

- Habla SMTP a mano (`smtp.go`): EHLO, MAIL FROM, RCPT TO, DATA.
- Clasifica cada respuesta (`bounce.go`): entregado, temporal, duro o reputación.
  El código extendido (RFC 3463) manda sobre el básico.
- Reintenta solo lo temporal, con backoff exponencial, y prueba los MX en orden de preferencia.
- Espacia las conexiones por proveedor (Gmail, Outlook, Yahoo, otros) aunque se envíe en paralelo.

## Siguientes pasos

1. Cola persistente con el próximo intento programado (en vez de `sleep`).
2. ~~Límite por proveedor~~ (hecho, `throttle.go`). Falta: bajar el ritmo solo cuando un proveedor devuelve 4.7.x.
3. STARTTLS y firma DKIM.
4. Pools de IPs y warmup simulados.
5. Una herramienta para depurar: por qué no llegó un correo, por destinatario, MX e intento.
