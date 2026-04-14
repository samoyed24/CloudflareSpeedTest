#!/bin/sh
set -eu

generate_result_yaml_from_env() {
  has_any=""
  if [ -n "${CFST_RESULT_ENVIRONMENT:-}" ] || [ -n "${CFST_HISTORY_HOURS:-}" ] || [ -n "${CFST_VMESS_TEMPLATE_B64:-}" ]; then
    has_any="1"
  fi

  if [ -z "${has_any}" ]; then
    return 0
  fi

  if [ -z "${CFST_RESULT_ENVIRONMENT:-}" ] || [ -z "${CFST_HISTORY_HOURS:-}" ] || [ -z "${CFST_VMESS_TEMPLATE_B64:-}" ]; then
    echo "[entrypoint] partial CFST_RESULT_* env detected, skip overriding /app/result.yaml" >&2
    return 0
  fi

  if ! echo "${CFST_HISTORY_HOURS}" | grep -Eq '^[0-9]+$'; then
    echo "[entrypoint] CFST_HISTORY_HOURS must be an integer, got: ${CFST_HISTORY_HOURS}" >&2
    exit 1
  fi

  vmess_tmp="$(mktemp)"
  if ! printf '%s' "${CFST_VMESS_TEMPLATE_B64}" | base64 -d > "${vmess_tmp}" 2>/dev/null; then
    rm -f "${vmess_tmp}"
    echo "[entrypoint] failed to decode CFST_VMESS_TEMPLATE_B64" >&2
    exit 1
  fi

  escaped_env="$(printf '%s' "${CFST_RESULT_ENVIRONMENT}" | sed 's/\\/\\\\/g; s/"/\\"/g')"
  {
    printf 'environment: "%s"\n' "${escaped_env}"
    printf 'history_hours: %s\n' "${CFST_HISTORY_HOURS}"
    printf 'vmess_template: |\n'
    sed 's/^/  /' "${vmess_tmp}"
  } > /app/result.yaml

  rm -f "${vmess_tmp}"
  echo "[entrypoint] /app/result.yaml generated from CFST_RESULT_* env"
}

write_tls_files_from_env() {
  has_any=""
  if [ -n "${NGINX_SSL_CERT_PEM_B64:-}" ] || [ -n "${NGINX_SSL_KEY_PEM_B64:-}" ]; then
    has_any="1"
  fi

  if [ -z "${has_any}" ]; then
    return 0
  fi

  if [ -z "${NGINX_SSL_CERT_PEM_B64:-}" ] || [ -z "${NGINX_SSL_KEY_PEM_B64:-}" ]; then
    echo "[entrypoint] partial NGINX_SSL_*_PEM_B64 env detected, skip writing TLS files" >&2
    return 0
  fi

  cert_dir="$(dirname "${NGINX_SSL_CERT_PATH}")"
  key_dir="$(dirname "${NGINX_SSL_KEY_PATH}")"
  mkdir -p "${cert_dir}" "${key_dir}"

  if ! printf '%s' "${NGINX_SSL_CERT_PEM_B64}" | base64 -d > "${NGINX_SSL_CERT_PATH}" 2>/dev/null; then
    echo "[entrypoint] failed to decode NGINX_SSL_CERT_PEM_B64" >&2
    exit 1
  fi
  if ! printf '%s' "${NGINX_SSL_KEY_PEM_B64}" | base64 -d > "${NGINX_SSL_KEY_PATH}" 2>/dev/null; then
    echo "[entrypoint] failed to decode NGINX_SSL_KEY_PEM_B64" >&2
    exit 1
  fi

  chmod 644 "${NGINX_SSL_CERT_PATH}"
  chmod 600 "${NGINX_SSL_KEY_PATH}"
  echo "[entrypoint] TLS cert/key written from NGINX_SSL_*_PEM_B64 env"
}

export NGINX_HTTP_PORT="${NGINX_HTTP_PORT:-8080}"
export NGINX_HTTPS_PORT="${NGINX_HTTPS_PORT:-8443}"
export NGINX_SERVER_NAME="${NGINX_SERVER_NAME:-_}"
export NGINX_SSL_CERT_PATH="${NGINX_SSL_CERT_PATH:-/app/certs/fullchain.pem}"
export NGINX_SSL_KEY_PATH="${NGINX_SSL_KEY_PATH:-/app/certs/privkey.pem}"

generate_result_yaml_from_env
write_tls_files_from_env

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
