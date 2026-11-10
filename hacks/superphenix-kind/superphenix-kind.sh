#!/usr/bin/env bash
# superphenix-kind.sh - run Superphenix in kind. Usage: ./superphenix-kind.sh <up|down|status|apply|dev|ca>
#
# Settings: see .env.example. Copy it to .env, or set them on the command line.
#
# The script keeps no state of its own: it reads everything back from the cluster.
# generated/ only holds the image overrides written by `dev`.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFESTS="${DIR}/manifests"
GEN="${DIR}/generated"   # git-ignored: dev image overrides
CA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/kind-superphenix/ca"   # kept across labs: trust it once

NAME="spx-lab"   # kind cluster and Docker network
CTX="kind-${NAME}"
NS="superphenix-system"
RELEASE="superphenix-operator"
CHART="oci://ghcr.io/super-phenix/charts/superphenix-operator"
DOMAIN="127.0.0.1.nip.io"
LOCAL_APPS="clusters.local.systemConfiguration.apps"
MGMT_APPS="management.systemConfiguration.apps"

# ---------------------------------------------------------------------------
# Small helpers
# ---------------------------------------------------------------------------
ok()   { echo "  ok    $*"; }
warn() { echo "  warn  $*"; }
die()  { echo "  error $*" >&2; exit 1; }
step() { echo; echo "==> $*"; }
k()    { kubectl --context "${CTX}" "$@"; }
exists() { kind get clusters 2>/dev/null | grep -qx "${NAME}"; }
port_in_use() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }   # true if something answers

usage() {
  cat <<EOF
Usage: ./superphenix-kind.sh <command>
  up        create the lab, or finish one that timed out
  down      delete the lab
  status    show what is not healthy, URLs and passwords
  apply     push the values files to the lab again
  dev <operator|controller|api|console|reset>
            build a component from your checkout and deploy it
  ca        show how to trust the lab's HTTPS certificate
EOF
}

# On macOS, Docker runs in a VM, so the kernel modules can't be loaded from here.
if [[ "$(uname -s)" == Darwin ]]; then
  die "macOS is not supported yet. Run the lab on a remote x86-64 Linux machine (see README.md)."
fi
if (( BASH_VERSINFO[0] < 4 )); then
  die "This script needs bash 4 or newer (found ${BASH_VERSION})."
fi

spx_clusters() { k -n "${NS}" get clusters.operator.superphenix.net -o json 2>/dev/null || echo '{"items":[]}'; }

all_deployed() {
  spx_clusters | jq -e '(.items | length) > 0 and all(.items[]; .status.phase == "Deployed"
    or any(.status.conditions[]?; .type == "Ready" and .status == "True"))' >/dev/null
}

# One progress line, for example "local: Deploying, 12/40 apps healthy".
progress() {
  spx_clusters | jq -r '[.items[] | (.status.apps // {}) as $a
    | "\(.metadata.name): \(.status.phase // "Pending"), \([$a[] | select(.status == "Healthy")] | length)/\($a | length) apps healthy"]
    | join("  |  ") | if . == "" then "waiting for the operator" else . end'
}

# Helm release helpers
release_values() { helm --kube-context "${CTX}" -n "${NS}" get values "${RELEASE}" -o json 2>/dev/null || echo '{}'; }
release_on_host_network() { release_values | jq -r '.installOnClusterWithoutCNI | tostring'; }
release_has_kaas() { release_values | jq -r 'if .clusters.local.systemConfiguration.apps["kaas-controller"].enabled == true then 1 else 0 end' 2>/dev/null || echo 0; }

# ---------------------------------------------------------------------------
# Settings: command line > .env > defaults below (for KAAS: what the lab runs).
# Sourcing .env would overwrite values from the command line, so save those
# first and put them back afterwards.
# ---------------------------------------------------------------------------
SETTINGS=(KAAS SPX_VERSION SUPERPHENIX_REPO CONSOLE_REPO KIND_NODE_IMAGE)
declare -A from_cli=()
for var in "${SETTINGS[@]}"; do
  if [[ -n "${!var:-}" ]]; then from_cli[${var}]="${!var}"; fi
done
# shellcheck disable=SC1091
if [[ -f "${DIR}/.env" ]]; then source "${DIR}/.env"; fi
for var in "${!from_cli[@]}"; do printf -v "${var}" '%s' "${from_cli[${var}]}"; done

SPX_VERSION="${SPX_VERSION:-latest}"
SUPERPHENIX_REPO="${SUPERPHENIX_REPO:-../..}"          # this checkout
CONSOLE_REPO="${CONSOLE_REPO:-../../../console-ui}"    # next to this checkout
KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.37.0}"
# KAAS set nowhere: keep what the lab runs, so `make up KAAS=1` then `make apply` keeps KaaS.
KAAS="${KAAS:-$(release_has_kaas)}"
[[ "${KAAS}" == 0 || "${KAAS}" == 1 ]] || die "KAAS must be 0 or 1 (got '${KAAS}')."
# Relative paths start from this folder.
if [[ "${SUPERPHENIX_REPO}" != /* ]]; then SUPERPHENIX_REPO="${DIR}/${SUPERPHENIX_REPO}"; fi
if [[ "${CONSOLE_REPO}" != /* ]]; then CONSOLE_REPO="${DIR}/${CONSOLE_REPO}"; fi

# ---------------------------------------------------------------------------
# Step 1: host
# ---------------------------------------------------------------------------
preflight() {
  local tool
  for tool in docker kind kubectl helm jq openssl; do
    command -v "${tool}" >/dev/null || die "${tool} is missing (see README.md)."
  done
  docker info >/dev/null 2>&1 || die "Docker is not running, or your user cannot use it."
  if [[ "$(uname -m)" != x86_64 ]]; then
    warn "$(uname -m) host: the released Superphenix images are amd64 only, so the operator will not start."
    warn "Build arm64 images with 'make dev-operator dev-controller dev-api dev-console' (untested)."
  fi

  local gib need=$(( KAAS == 1 ? 10 : 8 ))
  gib=$(( $(docker info --format '{{.MemTotal}}') / 1024**3 ))
  (( gib + 1 >= need )) || warn "Docker has ${gib} GiB of RAM, the lab wants about ${need} GiB."
  [[ -e /dev/kvm ]] || warn "/dev/kvm is missing: VMs will run emulated, which is slow."

  # kind nodes are containers on the host kernel: Kube-OVN needs these modules there.
  local mod missing=()
  for mod in openvswitch geneve macvlan; do [[ -d /sys/module/${mod} ]] || missing+=("${mod}"); done
  if (( ${#missing[@]} )); then
    sudo modprobe -a "${missing[@]}" || die "Could not load ${missing[*]}."
  fi

  # Many pods watch files: the default limits run out and pods fail at random.
  if (( $(sysctl -n fs.inotify.max_user_instances) < 8192 )); then
    sudo sysctl -q -w fs.inotify.max_user_instances=8192 fs.inotify.max_user_watches=524288
  fi
  ok "host is ready"
}

# ---------------------------------------------------------------------------
# Step 2: kind cluster
# ---------------------------------------------------------------------------
create_cluster() {
  # Plays the datacenter network: nodes in 172.30.0.0/24, NAT gateways and elastic IPs in the rest.
  if ! docker network inspect "${NAME}" >/dev/null 2>&1; then
    docker network create --driver bridge --ipv6 \
      --subnet 172.30.0.0/16 --ip-range 172.30.0.0/24 --gateway 172.30.0.1 \
      --subnet fd00:172:30::/64 "${NAME}" >/dev/null
  fi

  if ! exists; then
    local port
    for port in 80 443 7442 7443 7444; do   # the ports kind.yaml publishes
      if port_in_use "${port}"; then die "Port ${port} is already used on 127.0.0.1."; fi
    done
    # --wait 0s: nodes can't be Ready before Superphenix installs the CNI.
    KIND_EXPERIMENTAL_DOCKER_NETWORK="${NAME}" \
      kind create cluster --config "${MANIFESTS}/kind.yaml" --image "${KIND_NODE_IMAGE}" --wait 0s
    # Traefik runs on the control plane, where ports 80 and 443 are published.
    k taint nodes --all node-role.kubernetes.io/control-plane- >/dev/null 2>&1 || true
    # Remove kind's own storage; Superphenix brings local-path-provisioner.
    k delete storageclass standard --ignore-not-found >/dev/null
    k delete namespace local-path-storage --ignore-not-found --wait=false >/dev/null
  fi
  k apply -f "${MANIFESTS}/storage.yaml" >/dev/null
  ok "kind cluster ${NAME} is up"
}

# ---------------------------------------------------------------------------
# Step 3: install Superphenix
# ---------------------------------------------------------------------------

# Creates the lab CA once, then a fresh wildcard certificate for Traefik.
install_certificate() {
  if [[ ! -f "${CA_DIR}/ca.key" ]]; then
    mkdir -p "${CA_DIR}" && chmod 700 "${CA_DIR}"
    openssl req -x509 -new -nodes -newkey rsa:3072 -sha256 -days 3650 \
      -keyout "${CA_DIR}/ca.key" -out "${CA_DIR}/ca.crt" -subj "/CN=Superphenix lab CA ($(hostname))" \
      -addext "basicConstraints=critical,CA:TRUE,pathlen:0" -addext "keyUsage=critical,keyCertSign,cRLSign" \
      -addext "nameConstraints=critical,permitted;DNS:${DOMAIN}" 2>/dev/null
    warn "new lab CA: run 'make ca' to trust it in your browser"
  fi
  local tmp; tmp="$(mktemp -d)"
  openssl req -new -nodes -newkey rsa:2048 -keyout "${tmp}/tls.key" -subj "/CN=*.${DOMAIN}" 2>/dev/null \
    | openssl x509 -req -sha256 -days 397 -CA "${CA_DIR}/ca.crt" -CAkey "${CA_DIR}/ca.key" -CAcreateserial \
      -extfile <(printf 'subjectAltName=DNS:%s,DNS:*.%s,DNS:*.console.%s\nextendedKeyUsage=serverAuth\n' \
        "${DOMAIN}" "${DOMAIN}" "${DOMAIN}") -out "${tmp}/tls.crt" 2>/dev/null
  k get namespace traefik-system >/dev/null 2>&1 || k create namespace traefik-system >/dev/null
  k -n traefik-system create secret tls spx-lab-tls --cert "${tmp}/tls.crt" --key "${tmp}/tls.key" \
    --dry-run=client -o yaml | k apply -f - >/dev/null
  rm -rf "${tmp}"
}

# Fills the HELM_ARGS array: values files first, then --set flags (later wins).
helm_args() {
  local ip file
  ip="$(docker inspect "${NAME}-control-plane" -f "{{(index .NetworkSettings.Networks \"${NAME}\").IPAddress}}")"

  # A new lab starts on the host network, because there is no CNI yet.
  local host_network=true
  if [[ "$(release_on_host_network)" == false ]] || all_deployed; then host_network=false; fi

  HELM_ARGS=(-f "${MANIFESTS}/values.yaml")
  if (( KAAS == 1 )); then HELM_ARGS+=(-f "${MANIFESTS}/kaas.yaml"); fi
  if [[ -f "${DIR}/local.yaml" ]]; then HELM_ARGS+=(-f "${DIR}/local.yaml"); fi
  HELM_ARGS+=(
    --set "installOnClusterWithoutCNI=${host_network}"
    --set "${LOCAL_APPS}.kubeovn.values.masterNodes=${ip}"
    # KaaS worker VMs reach Traefik on the control-plane IP: 127.0.0.1 would be the VM itself.
    --set "${MGMT_APPS}.superphenix-api.values.config.productsConfig.argoApp.kubernetes.azDomains.local.internal=${ip}.nip.io"
  )
  if [[ ! -e /dev/kvm ]]; then
    HELM_ARGS+=(--set "${LOCAL_APPS}.kubevirt-config.values.config.configuration.developerConfiguration.useEmulation=true")
  fi
  # Images built by `dev`: one Helm argument per line in generated/dev-<component>.args.
  for file in "${GEN}"/dev-*.args; do
    if [[ -f "${file}" ]]; then mapfile -t -O "${#HELM_ARGS[@]}" HELM_ARGS < "${file}"; fi
  done
}

cmd_apply() {
  exists || die "No lab yet. Run 'make up' first."
  if [[ "${SPX_VERSION}" == latest ]]; then
    SPX_VERSION="$(helm show chart "${CHART}" | awk '/^version:/ {print $2}')"
  fi
  install_certificate
  helm_args

  # The released chart, or the one in your checkout after `dev operator`.
  local chart=("${CHART}" --version "${SPX_VERSION}")
  if [[ -f "${GEN}/dev-operator.args" ]]; then
    chart=("${SUPERPHENIX_REPO}/components/system/superphenix-operator" --set "config.system.version=${SPX_VERSION}")
  fi

  # The chart also defines its namespace: this label and annotation let Helm adopt it.
  k get namespace "${NS}" >/dev/null 2>&1 || k create namespace "${NS}" >/dev/null
  k label namespace "${NS}" app.kubernetes.io/managed-by=Helm --overwrite >/dev/null
  k annotate namespace "${NS}" meta.helm.sh/release-name="${RELEASE}" meta.helm.sh/release-namespace="${NS}" --overwrite >/dev/null

  local out
  out="$(helm --kube-context "${CTX}" upgrade --install "${RELEASE}" -n "${NS}" "${chart[@]}" "${HELM_ARGS[@]}" 2>&1)" \
    || { echo "${out}" >&2; die "helm upgrade failed."; }
  if (( KAAS == 1 )); then ok "Superphenix ${SPX_VERSION} applied with KaaS"; else ok "Superphenix ${SPX_VERSION} applied"; fi
}

# ---------------------------------------------------------------------------
# Step 4: wait
# ---------------------------------------------------------------------------

# Returns 1 if the lab is still not ready after the time limit.
wait_deployed() {
  echo "  Kube-OVN first, then every component. The first run takes 20 to 40 minutes."
  local start=${SECONDS}
  until k wait --for=condition=Ready nodes --all --timeout=10s >/dev/null 2>&1; do
    if (( SECONDS - start > 1200 )); then
      warn "nodes are still not Ready after 20 minutes: check Kube-OVN with 'make status'."
      return 1
    fi
    echo "  $(date +%H:%M:%S)  waiting for nodes to be Ready (Kube-OVN)"
    sleep 20
  done
  ok "nodes are Ready"

  start=${SECONDS}
  while (( SECONDS - start < 3600 )); do
    if all_deployed; then ok "every cluster is Deployed"; return 0; fi
    echo "  $(date +%H:%M:%S)  $(progress)"
    sleep 30
  done
  return 1
}

# ---------------------------------------------------------------------------
# Commands
# ---------------------------------------------------------------------------
cmd_up() {
  step "Checking the host";       preflight
  step "Creating the cluster";    create_cluster
  step "Installing Superphenix";  cmd_apply
  step "Waiting for Superphenix"
  if ! wait_deployed; then
    warn "still deploying. Watch 'make status', then run 'make up' again to finish."
    return 0
  fi
  # Everything is up: apply again, so the operator and ArgoCD leave the host network.
  if [[ "$(release_on_host_network)" == true ]]; then
    step "Moving the operator and ArgoCD to the pod network"
    cmd_apply
  fi
  cmd_info
}

cmd_down() {
  if exists; then kind delete cluster --name "${NAME}"; fi
  docker network rm "${NAME}" >/dev/null 2>&1 || true
  rm -rf "${GEN}"
  ok "lab deleted (the CA in ${CA_DIR} is kept)"
}

cmd_info() {
  local pw
  pw="$(k -n "${NS}" get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' 2>/dev/null | base64 -d || true)"
  cat <<EOF

  Console   https://console.${DOMAIN}   (sign up with any email)
  ArgoCD    https://argocd.${DOMAIN}    (admin / ${pw:-not ready yet})
  Grafana   https://grafana.${DOMAIN}
  kubectl   kubectl --context ${CTX}

EOF
}

cmd_status() {
  exists || die "No lab yet. Run 'make up' first."
  step "Superphenix clusters"
  k -n "${NS}" get clusters.operator.superphenix.net || true
  step "ArgoCD applications not Synced and Healthy"
  k -n "${NS}" get applications.argoproj.io -o json | jq -r '.items[]
    | select(.status.sync.status != "Synced" or .status.health.status != "Healthy")
    | [.metadata.name, .status.sync.status // "-", .status.health.status // "-",
       (.status.conditions[0].message // .status.operationState.message // "" | .[0:100])] | @tsv' | column -t -s $'\t'
  step "Pods not ready"
  # Pods not Running, or Running with fewer ready containers than total. Skip finished jobs.
  k get pods -A --no-headers | awk '$4 != "Completed" { split($3, r, "/"); if ($4 != "Running" || r[1] != r[2]) print }'
  cmd_info
}

# Builds one component from your checkout, loads it into the kind nodes, and points Helm at it.
cmd_dev() {
  local component="${1:-}" values setting=SUPERPHENIX_REPO
  local repo="${SUPERPHENIX_REPO}" dockerfile="${SUPERPHENIX_REPO}/components/system/superphenix-${component}/docker/Dockerfile"
  case "${component}" in
    operator)   values="" ;;
    controller) values="${LOCAL_APPS}.superphenix-controller.values." ;;
    api)        values="${MGMT_APPS}.superphenix-api.values." ;;
    console)    values="${MGMT_APPS}.superphenix-console.values." setting=CONSOLE_REPO
                repo="${CONSOLE_REPO}" dockerfile="${CONSOLE_REPO}/Dockerfile" ;;
    reset)      rm -f "${GEN}"/dev-*.args; cmd_apply; return ;;
    *) die "Usage: ./superphenix-kind.sh dev <operator|controller|api|console|reset>" ;;
  esac
  [[ -f "${dockerfile}" ]] || die "No ${component} checkout at ${repo}. Set ${setting} (see .env.example)."

  local image="localhost/superphenix-${component}" tag; tag="dev-$(date +%Y%m%d%H%M%S)"
  docker build -t "${image}:${tag}" --build-arg "VERSION=${SPX_VERSION}" -f "${dockerfile}" "${repo}"

  # Load the image straight into each node's containerd (what `kind load` does).
  local node
  for node in $(kind get nodes --name "${NAME}"); do
    docker save "${image}:${tag}" | docker exec -i "${node}" ctr --namespace=k8s.io images import --digests - >/dev/null \
      || die "Could not load ${image}:${tag} on ${node}."
  done
  if [[ "${component}" == operator ]]; then
    k apply --server-side --force-conflicts -f "${SUPERPHENIX_REPO}/components/system/superphenix-operator/crds" >/dev/null
  fi
  mkdir -p "${GEN}"
  printf '%s\n' --set "${values}image.repository=${image}" --set "${values}image.tag=${tag}" > "${GEN}/dev-${component}.args"
  cmd_apply
  ok "${component} runs ${image}:${tag}; ArgoCD rolls it out in a minute or two"
}

cmd_ca() {
  [[ -f "${CA_DIR}/ca.crt" ]] || die "No lab CA yet. Run 'make up' first."
  echo "Trust ${CA_DIR}/ca.crt once. It can only sign names under ${DOMAIN}."
  if grep -qi microsoft /proc/version 2>/dev/null; then
    echo "  Windows (PowerShell): Import-Certificate -FilePath '$(wslpath -w "${CA_DIR}/ca.crt")' -CertStoreLocation Cert:\\CurrentUser\\Root"
  fi
  echo "  Linux: sudo cp ${CA_DIR}/ca.crt /usr/local/share/ca-certificates/superphenix-lab.crt && sudo update-ca-certificates"
  echo "  Firefox: Settings > Privacy & Security > Certificates > View Certificates > Authorities > Import"
}

case "${1:-}" in
  up)     cmd_up ;;
  down)   cmd_down ;;
  status) cmd_status ;;
  apply)  cmd_apply ;;
  dev)    cmd_dev "${2:-}" ;;
  ca)     cmd_ca ;;
  *)      usage; exit 1 ;;
esac
