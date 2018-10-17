# API

## Landingpage

```http
GET / HTTP/1.1
Host: cion.local
Content-Type: text/html; charset=utf-8
Accept: text/html
```

## Zone Registration

```http
PUT /register/:zone HTTP/1.1
Host: cion.local
Accept: application/json; version=1.0.0
Content-Type: application/json
```

## Add SRV record

```http
POST /zone/:zone
Host: cion.local
Accept: application/json; version=1.0.0
X-Cion-Auth-Key: <secret>
X-Cion-Srv: matrix
X-Cion-Proto: tcp
X-Cion-Prio: 10
X-Cion-Weight: 0
X-Cion-Port: 8448
X-Cion-Dest: 127.0.0.1
```

## Update SRV record

```http
POST /zone/:zone
Host: cion.local
Content-Type: application/json; version=1.0.0
X-Cion-Auth-Key: <secret>
X-Cion-Srv: matrix
X-Cion-Proto: tcp
X-Cion-Priority: 10
X-Cion-Weight: 0
X-Cion-Port: 8448
X-Cion-Dest: 127.0.0.1
```

## Delete SRV record

```http
DELETE /zone/:zone
Host: cion.local
Content-Type: application/json; version=1.0.0
X-Cion-Auth-Key: <secret>
X-Cion-Srv: matrix
X-Cion-Proto: tcp
X-Cion-Priority: 10
X-Cion-Weight: 0
X-Cion-Port: 8448
X-Cion-Dest: 127.0.0.1
```
