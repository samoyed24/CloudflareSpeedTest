#!/bin/sh
set -eu

export NGINX_HTTP_PORT="${NGINX_HTTP_PORT:-8080}"
export NGINX_HTTPS_PORT="${NGINX_HTTPS_PORT:-8443}"
export NGINX_SERVER_NAME="${NGINX_SERVER_NAME:-_}"
export NGINX_SSL_CERT_PATH="${NGINX_SSL_CERT_PATH:-/app/certs/fullchain.pem}"
export NGINX_SSL_KEY_PATH="${NGINX_SSL_KEY_PATH:-/app/certs/privkey.pem}"

mkdir -p /etc/nginx/http.d

if [ -f "${NGINX_SSL_CERT_PATH}" ] && [ -f "${NGINX_SSL_KEY_PATH}" ]; then
  envsubst '${NGINX_HTTP_PORT} ${NGINX_HTTPS_PORT} ${NGINX_SERVER_NAME} ${NGINX_SSL_CERT_PATH} ${NGINX_SSL_KEY_PATH}' \
    < /etc/nginx/templates/default.conf.template \
    > /etc/nginx/http.d/default.conf
  echo "[entrypoint] nginx configured for HTTP:${NGINX_HTTP_PORT} + HTTPS:${NGINX_HTTPS_PORT}"
else
  envsubst '${NGINX_HTTP_PORT} ${NGINX_SERVER_NAME}' \
    < /etc/nginx/templates/http-only.conf.template \
    > /etc/nginx/http.d/default.conf
  echo "[entrypoint] cert/key not found, nginx configured HTTP only on :${NGINX_HTTP_PORT}"
fi

/docker/run-cfst-loop.sh &

exec nginx -g 'daemon off;'
