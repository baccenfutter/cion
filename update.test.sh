CIO_AUTH_KEY="${CION_AUTH_KEY}"

set -e

#curl \
#  -X POST \
#  -H "Accept: application/json; version=1.0.0" \
#  -H "Content-Type: application/json" \
#  -H "X-Cion-Auth-Key: ${CION_AUTH_KEY}" \
#  -H "X-Cion-Update-Type: A" \
#  -d '{"hostname":"matrix","address":"127.0.0.1"}' \
#  http://localhost:1234/zone/example

curl \
  -X POST \
  -H "Accept: application/json; version=1.0.0" \
  -H "Content-Type: application/json" \
  -H "X-Cion-Auth-Key: ${CION_AUTH_KEY}" \
  -H "X-Cion-Update-Type: SRV" \
  -d '{"srv":"matrix","proto":"tcp","prio":10,"weight":0,"port":8448,"dest":"matrix.example.foo.bar."}' \
  http://localhost:1234/zone/example
