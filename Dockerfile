FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags "-s -w" -o /out/cfst .

FROM alpine:3.20

RUN apk add --no-cache nginx gettext

WORKDIR /app

COPY --from=builder /out/cfst /usr/local/bin/cfst
COPY ip.txt ipv6.txt result.yaml ./
COPY docker/entrypoint.sh /docker/entrypoint.sh
COPY docker/run-cfst-loop.sh /docker/run-cfst-loop.sh
COPY docker/nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY docker/nginx/http-only.conf.template /etc/nginx/templates/http-only.conf.template

RUN chmod +x /docker/entrypoint.sh /docker/run-cfst-loop.sh \
	&& rm -f /etc/nginx/http.d/default.conf \
	&& mkdir -p /var/lib/nginx/tmp /run/nginx /app/certs

ENV CFST_INTERVAL_SECONDS=300 \
	CFST_ARGS="-f ip.txt" \
	NGINX_HTTP_PORT=8080 \
	NGINX_HTTPS_PORT=8443 \
	NGINX_SERVER_NAME=_ \
	NGINX_SSL_CERT_PATH=/app/certs/fullchain.pem \
	NGINX_SSL_KEY_PATH=/app/certs/privkey.pem

EXPOSE 8080 8443

ENTRYPOINT ["/docker/entrypoint.sh"]
