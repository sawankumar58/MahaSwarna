#!/usr/bin/env bash
# ─── MahaSwarna — JWT RS256 Zero-Downtime Key Rotation ─────────────────────
#
# Rotates the JWT RS256 signing key without invalidating existing in-flight
# tokens. Follows the architecture decision recorded in ADR-003 (RS256 with
# zero-downtime rotation).
#
# Strategy:
#   1. Generate new RSA-2048 keypair (new_private.key / new_public.key).
#   2. Update the backend env: JWT_PUBLIC_KEY = old_public + "\n" + new_public
#      — both keys are accepted for verification during the overlap window.
#   3. Update JWT_PRIVATE_KEY to new_private (new tokens signed with new key).
#   4. Restart services (rolling restart — zero downtime).
#   5. Wait for the configured TOKEN_EXPIRY_MINUTES (default: 60) to pass.
#   6. Remove the old public key from JWT_PUBLIC_KEY (single-key mode again).
#   7. Optionally restart again to flush any verification cache.
#
# Prerequisites:
#   - openssl
#   - kubectl (if deploying on Kubernetes)
#   - OR docker (if deploying with docker-compose)
#   - ROTATE_MODE env var: "k8s" (default) or "compose"
#   - K8S_NAMESPACE (for k8s mode, default: mahaswarna)
#   - TOKEN_EXPIRY_MINUTES (overlap window, default: 60)
#
# Usage:
#   ROTATE_MODE=compose ./scripts/jwt_rotate.sh
#   ROTATE_MODE=k8s K8S_NAMESPACE=production ./scripts/jwt_rotate.sh
#
# CRITICAL: Run this script from the mahaswarna_backend/ directory.
# ────────────────────────────────────────────────────────────────────────────

set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────

ROTATE_MODE="${ROTATE_MODE:-k8s}"
K8S_NAMESPACE="${K8S_NAMESPACE:-mahaswarna}"
TOKEN_EXPIRY_MINUTES="${TOKEN_EXPIRY_MINUTES:-60}"
KEY_DIR="${KEY_DIR:-./.jwt_rotation}"
SECRET_NAME="${SECRET_NAME:-mahaswarna-jwt-keys}"   # k8s secret name

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
NEW_PRIVATE_KEY="${KEY_DIR}/jwt_private_${TIMESTAMP}.pem"
NEW_PUBLIC_KEY="${KEY_DIR}/jwt_public_${TIMESTAMP}.pem"

# ── Colours ─────────────────────────────────────────────────────────────────

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
BLUE='\033[0;34m'; BOLD='\033[1m'; RESET='\033[0m'

info()    { echo -e "${BLUE}[INFO]${RESET}  $*"; }
success() { echo -e "${GREEN}[OK]${RESET}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${RESET}  $*"; }
fatal()   { echo -e "${RED}[FATAL]${RESET} $*" >&2; exit 1; }

# ── Preflight checks ────────────────────────────────────────────────────────

info "Preflight checks..."

command -v openssl > /dev/null || fatal "openssl not found"

if [[ "${ROTATE_MODE}" == "k8s" ]]; then
  command -v kubectl > /dev/null || fatal "kubectl not found (required for ROTATE_MODE=k8s)"
  kubectl get namespace "${K8S_NAMESPACE}" > /dev/null 2>&1 \
    || fatal "Kubernetes namespace '${K8S_NAMESPACE}' not found or no cluster access"
elif [[ "${ROTATE_MODE}" == "compose" ]]; then
  command -v docker > /dev/null || fatal "docker not found (required for ROTATE_MODE=compose)"
else
  fatal "Unknown ROTATE_MODE='${ROTATE_MODE}'. Use 'k8s' or 'compose'."
fi

mkdir -p "${KEY_DIR}"
chmod 700 "${KEY_DIR}"

success "Preflight passed"

# ── Step 1: Generate new keypair ────────────────────────────────────────────

info "Step 1/6 — Generating new RSA-2048 keypair..."

openssl genrsa -out "${NEW_PRIVATE_KEY}" 2048 2>/dev/null
openssl rsa -in "${NEW_PRIVATE_KEY}" -pubout -out "${NEW_PUBLIC_KEY}" 2>/dev/null
chmod 600 "${NEW_PRIVATE_KEY}" "${NEW_PUBLIC_KEY}"

success "New keypair: ${NEW_PRIVATE_KEY}"

# ── Fetch current public key ────────────────────────────────────────────────

info "Step 2/6 — Fetching current JWT_PUBLIC_KEY from ${ROTATE_MODE}..."

if [[ "${ROTATE_MODE}" == "k8s" ]]; then
  CURRENT_PUBLIC=$(kubectl get secret "${SECRET_NAME}" \
    -n "${K8S_NAMESPACE}" \
    -o jsonpath='{.data.JWT_PUBLIC_KEY}' \
    | base64 -d)
else
  # docker-compose: read from .env
  [[ -f .env ]] || fatal ".env not found (run 'make env' first)"
  CURRENT_PUBLIC=$(grep '^JWT_PUBLIC_KEY=' .env | cut -d= -f2- | tr -d '"')
fi

[[ -n "${CURRENT_PUBLIC}" ]] || fatal "JWT_PUBLIC_KEY is empty — cannot determine old key"

# Dual-key value: old + new, separated by newline
# Services parse this as a list and accept tokens signed by either key.
DUAL_PUBLIC="${CURRENT_PUBLIC}
$(cat "${NEW_PUBLIC_KEY}")"

# ── Step 3: Update to dual-key (accept both old and new) ────────────────────

info "Step 3/6 — Setting dual-key verification (old + new public keys)..."

if [[ "${ROTATE_MODE}" == "k8s" ]]; then
  kubectl patch secret "${SECRET_NAME}" \
    -n "${K8S_NAMESPACE}" \
    --type merge \
    -p "{\"stringData\":{\"JWT_PUBLIC_KEY\":$(echo "${DUAL_PUBLIC}" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')}}"
else
  # Update .env with dual public key (Python for safe multiline handling)
  python3 - <<PYEOF
import re, pathlib

env_path = pathlib.Path('.env')
content  = env_path.read_text()

dual_pub = open('${KEY_DIR}/dual_public_${TIMESTAMP}.pem','w')
dual_pub.write("""${DUAL_PUBLIC}""")
dual_pub.close()

# Replace JWT_PUBLIC_KEY line (may be multiline — use a sentinel file approach)
# For simplicity in compose mode, base64-encode the value to avoid newline issues.
import base64
encoded = base64.b64encode("""${DUAL_PUBLIC}""".encode()).decode()

content = re.sub(r'^JWT_PUBLIC_KEY=.*$', f'JWT_PUBLIC_KEY_B64={encoded}', content, flags=re.MULTILINE)
env_path.write_text(content)
print("Updated .env with dual public key (base64-encoded)")
PYEOF
fi

success "Dual-key verification activated"

# ── Step 4: Switch signing to new private key ────────────────────────────────

info "Step 4/6 — Switching JWT_PRIVATE_KEY to new key..."

NEW_PRIVATE_CONTENT=$(cat "${NEW_PRIVATE_KEY}")

if [[ "${ROTATE_MODE}" == "k8s" ]]; then
  kubectl patch secret "${SECRET_NAME}" \
    -n "${K8S_NAMESPACE}" \
    --type merge \
    -p "{\"stringData\":{\"JWT_PRIVATE_KEY\":$(echo "${NEW_PRIVATE_CONTENT}" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')}}"

  info "Rolling restart of all deployments to pick up new private key..."
  for deploy in mahaswarna-gateway mahaswarna-core mahaswarna-pricing mahaswarna-intelligence; do
    kubectl rollout restart deployment/"${deploy}" -n "${K8S_NAMESPACE}" || true
  done
  kubectl rollout status deployment/mahaswarna-gateway -n "${K8S_NAMESPACE}" --timeout=120s
else
  # compose: update .env
  python3 - "${NEW_PRIVATE_KEY}" <<'PYEOF'
import re, pathlib, sys, base64
env_path = pathlib.Path('.env')
content  = env_path.read_text()
new_priv = pathlib.Path(sys.argv[1]).read_text()
encoded  = base64.b64encode(new_priv.encode()).decode()
content  = re.sub(r'^JWT_PRIVATE_KEY.*$', f'JWT_PRIVATE_KEY_B64={encoded}', content, flags=re.MULTILINE)
env_path.write_text(content)
print("Updated .env with new private key (base64-encoded)")
PYEOF
  docker compose restart gateway core pricing intelligence
fi

success "New private key active — new tokens now signed with the new key"

# ── Step 5: Wait for token expiry overlap window ─────────────────────────────

OVERLAP_SECONDS=$((TOKEN_EXPIRY_MINUTES * 60))
warn "Step 5/6 — Waiting ${TOKEN_EXPIRY_MINUTES} minutes for old tokens to expire..."
warn "  (Press Ctrl+C to skip — only safe if you are certain no old tokens are in use)"

for ((i=OVERLAP_SECONDS; i>0; i-=30)); do
  echo -e "  ${YELLOW}${i}s remaining...${RESET}"
  sleep 30
done

success "Overlap window complete — removing old public key"

# ── Step 6: Remove old public key ────────────────────────────────────────────

info "Step 6/6 — Switching to single-key verification (new key only)..."

NEW_PUBLIC_CONTENT=$(cat "${NEW_PUBLIC_KEY}")

if [[ "${ROTATE_MODE}" == "k8s" ]]; then
  kubectl patch secret "${SECRET_NAME}" \
    -n "${K8S_NAMESPACE}" \
    --type merge \
    -p "{\"stringData\":{\"JWT_PUBLIC_KEY\":$(echo "${NEW_PUBLIC_CONTENT}" | python3 -c 'import json,sys; print(json.dumps(sys.stdin.read()))')}}"
  kubectl rollout restart deployment/mahaswarna-core    -n "${K8S_NAMESPACE}" || true
  kubectl rollout restart deployment/mahaswarna-gateway -n "${K8S_NAMESPACE}" || true
else
  python3 - "${NEW_PUBLIC_KEY}" <<'PYEOF'
import re, pathlib, sys, base64
env_path = pathlib.Path('.env')
content  = env_path.read_text()
new_pub  = pathlib.Path(sys.argv[1]).read_text()
encoded  = base64.b64encode(new_pub.encode()).decode()
content  = re.sub(r'^JWT_PUBLIC_KEY.*$', f'JWT_PUBLIC_KEY_B64={encoded}', content, flags=re.MULTILINE)
env_path.write_text(content)
PYEOF
  docker compose restart gateway core
fi

success "Key rotation complete!"
echo ""
echo -e "${BOLD}Summary${RESET}"
echo -e "  New private key: ${NEW_PRIVATE_KEY}"
echo -e "  New public key:  ${NEW_PUBLIC_KEY}"
echo -e "  Mode:            ${ROTATE_MODE}"
echo -e "  Overlap window:  ${TOKEN_EXPIRY_MINUTES} min"
echo ""
warn "Store the new private key in your secrets manager and delete the local copy:"
echo -e "  rm -f ${NEW_PRIVATE_KEY}"
