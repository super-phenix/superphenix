#!/usr/bin/env bash
# superphenix-kind.sh - run Superphenix in kind. The Makefile only calls this script.
#
# Settings come from the command line, then .env, then the defaults below
# (see .env.example).
#
# The script keeps no state of its own: it reads everything back from the
# cluster. The only files it writes are generated/dev-*.args, the image
# overrides from `dev`.
set -euo pipefail

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

# =============================================================================
# Constants
# =============================================================================

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFESTS="${DIR}/manifests"
GENERATED="${DIR}/generated"                                        # git-ignored
CA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/kind-superphenix/ca"  # kept across labs: trust it once

CLUSTER_NAME="spx-lab"     # also the name of the Docker network
KUBE_CONTEXT="kind-${CLUSTER_NAME}"
NAMESPACE="superphenix-system"
RELEASE="superphenix-operator"
CHART="oci://ghcr.io/super-phenix/charts/superphenix-operator"
DOMAIN="127.0.0.1.nip.io"

# Prefixes of the Helm values for the apps of each Superphenix cluster.
LOCAL_APPS="clusters.local.systemConfiguration.apps"
MANAGEMENT_APPS="management.systemConfiguration.apps"

# =============================================================================
# Output and small helpers
# =============================================================================

ok()   { echo "  ok    $*"; }
warn() { echo "  warn  $*"; }
die()  { echo "  error $*" >&2; exit 1; }
step() { echo; echo "==> $*"; }

k() { kubectl --context "${KUBE_CONTEXT}" "$@"; }

cluster_exists() { kind get clusters 2>/dev/null | grep -qx "${CLUSTER_NAME}"; }

# True if something listens on this port of 127.0.0.1.
port_in_use() { (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }

# =============================================================================
# Platform checks
# =============================================================================

# On macOS, Docker runs in a VM, so the kernel modules can't be loaded from here.
if [[ "$(uname -s)" == Darwin ]]; then
  die "macOS is not supported yet. Run the lab on a remote x86-64 Linux machine (see README.md)."
fi
if (( BASH_VERSINFO[0] < 4 )); then
  die "This script needs bash 4 or newer (found ${BASH_VERSION})."
fi

# =============================================================================
# Reading the lab's state
# =============================================================================

# The Superphenix Cluster resources, as JSON. Empty list if there are none yet.
spx_clusters() {
  k -n "${NAMESPACE}" get clusters.operator.superphenix.net -o json 2>/dev/null \
    || echo '{"items":[]}'
}

# True once every Superphenix cluster is Deployed (or Ready).
all_deployed() {
  spx_clusters | jq -e '
    (.items | length) > 0
    and all(.items[];
      .status.phase == "Deployed"
      or any(.status.conditions[]?; .type == "Ready" and .status == "True"))
  ' >/dev/null
}

# One progress line, for example "local: Deploying, 12/40 apps healthy".
progress() {
  spx_clusters | jq -r '
    [ .items[]
      | (.status.apps // {}) as $apps
      | ([$apps[] | select(.status == "Healthy")] | length) as $healthy
      | "\(.metadata.name): \(.status.phase // "Pending"), \($healthy)/\($apps | length) apps healthy"
    ]
    | join("  |  ")
    | if . == "" then "waiting for the operator" else . end
  '
}

# The values of the installed Helm release, as JSON. {} if not installed.
release_values() {
  helm --kube-context "${KUBE_CONTEXT}" -n "${NAMESPACE}" get values "${RELEASE}" -o json 2>/dev/null \
    || echo '{}'
}

# Prints "true" or "false", or "null" if the release is not installed.
release_on_host_network() {
  release_values | jq -r '.installOnClusterWithoutCNI | tostring'
}

# Prints 1 if the installed release has KaaS, 0 otherwise.
release_has_kaas() {
  release_values \
    | jq -r '
        if .clusters.local.systemConfiguration.apps["kaas-controller"].enabled == true
        then 1 else 0 end' 2>/dev/null \
    || echo 0
}

# =============================================================================
# Settings: command line > .env > defaults
# =============================================================================

# Sourcing .env would overwrite values from the command line, so save those
# first and put them back afterwards.
SETTINGS=(KAAS SPX_VERSION SUPERPHENIX_REPO CONSOLE_REPO KIND_NODE_IMAGE)
declare -A from_command_line=()
for var in "${SETTINGS[@]}"; do
  if [[ -n "${!var:-}" ]]; then
    from_command_line[${var}]="${!var}"
  fi
done

if [[ -f "${DIR}/.env" ]]; then
  # shellcheck disable=SC1091
  source "${DIR}/.env"
fi

for var in "${!from_command_line[@]}"; do
  printf -v "${var}" '%s' "${from_command_line[${var}]}"
done

SPX_VERSION="${SPX_VERSION:-0.9.0}"
SUPERPHENIX_REPO="${SUPERPHENIX_REPO:-../..}"            # this checkout
CONSOLE_REPO="${CONSOLE_REPO:-../../../console-ui}"      # next to this checkout
KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.37.0}"

# KAAS set nowhere: keep what the lab runs, so `make up KAAS=1` then
# `make apply` keeps KaaS.
KAAS="${KAAS:-$(release_has_kaas)}"
if [[ "${KAAS}" != 0 && "${KAAS}" != 1 ]]; then
  die "KAAS must be 0 or 1 (got '${KAAS}')."
fi

# Relative paths start from this folder.
if [[ "${SUPERPHENIX_REPO}" != /* ]]; then SUPERPHENIX_REPO="${DIR}/${SUPERPHENIX_REPO}"; fi
if [[ "${CONSOLE_REPO}" != /* ]]; then CONSOLE_REPO="${DIR}/${CONSOLE_REPO}"; fi

# =============================================================================
# Step 1: prepare the host
# =============================================================================

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

  local docker_gib needed_gib=8
  docker_gib=$(( $(docker info --format '{{.MemTotal}}') / 1024**3 ))
  if (( KAAS == 1 )); then needed_gib=10; fi
  if (( docker_gib + 1 < needed_gib )); then
    warn "Docker has ${docker_gib} GiB of RAM, the lab wants about ${needed_gib} GiB."
  fi

  if [[ ! -e /dev/kvm ]]; then
    warn "/dev/kvm is missing: VMs will run emulated, which is slow."
  fi

  # kind nodes are containers on the host kernel: Kube-OVN needs these modules there.
  local module missing=()
  for module in openvswitch geneve macvlan; do
    if [[ ! -d "/sys/module/${module}" ]]; then missing+=("${module}"); fi
  done
  if (( ${#missing[@]} > 0 )); then
    sudo modprobe -a "${missing[@]}" || die "Could not load ${missing[*]}."
  fi

  # Many pods watch files: the default limits run out and pods fail at random.
  if (( $(sysctl -n fs.inotify.max_user_instances) < 8192 )); then
    sudo sysctl -q -w fs.inotify.max_user_instances=8192 fs.inotify.max_user_watches=524288
  fi

  ok "host is ready"
}

# =============================================================================
# Step 2: create the kind cluster
# =============================================================================

create_cluster() {
  # This Docker network plays the datacenter network:
  # nodes get 172.30.0.0/24, NAT gateways and elastic IPs get the rest.
  if ! docker network inspect "${CLUSTER_NAME}" >/dev/null 2>&1; then
    docker network create --driver bridge --ipv6 \
      --subnet 172.30.0.0/16 --ip-range 172.30.0.0/24 --gateway 172.30.0.1 \
      --subnet fd00:172:30::/64 \
      "${CLUSTER_NAME}" >/dev/null
  fi

  if ! cluster_exists; then
    # The ports kind.yaml publishes on 127.0.0.1.
    local port
    for port in 80 443 7442 7443 7444; do
      if port_in_use "${port}"; then die "Port ${port} is already used on 127.0.0.1."; fi
    done

    # --wait 0s: nodes can't be Ready before Superphenix installs the CNI.
    KIND_EXPERIMENTAL_DOCKER_NETWORK="${CLUSTER_NAME}" \
      kind create cluster --config "${MANIFESTS}/kind.yaml" --image "${KIND_NODE_IMAGE}" --wait 0s

    # Traefik runs on the control plane, where ports 80 and 443 are published.
    k taint nodes --all node-role.kubernetes.io/control-plane- >/dev/null 2>&1 || true

    # Remove kind's own storage: Superphenix brings local-path-provisioner.
    k delete storageclass standard --ignore-not-found >/dev/null
    k delete namespace local-path-storage --ignore-not-found --wait=false >/dev/null
  fi

  k apply -f "${MANIFESTS}/storage.yaml" >/dev/null
  ok "kind cluster ${CLUSTER_NAME} is up"
}

# =============================================================================
# Step 3: install Superphenix
# =============================================================================

# Creates the lab CA once, then a fresh wildcard certificate for Traefik.
install_certificate() {
  if [[ ! -f "${CA_DIR}/ca.key" ]]; then
    mkdir -p "${CA_DIR}"
    chmod 700 "${CA_DIR}"
    # nameConstraints: this CA can only sign names under ${DOMAIN}.
    openssl req -x509 -new -nodes -newkey rsa:3072 -sha256 -days 3650 \
      -keyout "${CA_DIR}/ca.key" -out "${CA_DIR}/ca.crt" \
      -subj "/CN=Superphenix lab CA ($(hostname))" \
      -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
      -addext "keyUsage=critical,keyCertSign,cRLSign" \
      -addext "nameConstraints=critical,permitted;DNS:${DOMAIN}" 2>/dev/null
    warn "new lab CA: run 'make ca' to trust it in your browser"
  fi

  local tmp extensions
  tmp="$(mktemp -d)"
  extensions="subjectAltName=DNS:${DOMAIN},DNS:*.${DOMAIN},DNS:*.console.${DOMAIN}
extendedKeyUsage=serverAuth"

  openssl req -new -nodes -newkey rsa:2048 -keyout "${tmp}/tls.key" -subj "/CN=*.${DOMAIN}" 2>/dev/null \
    | openssl x509 -req -sha256 -days 397 \
        -CA "${CA_DIR}/ca.crt" -CAkey "${CA_DIR}/ca.key" -CAcreateserial \
        -extfile <(echo "${extensions}") -out "${tmp}/tls.crt" 2>/dev/null

  k get namespace traefik-system >/dev/null 2>&1 || k create namespace traefik-system >/dev/null
  k -n traefik-system create secret tls spx-lab-tls --cert "${tmp}/tls.crt" --key "${tmp}/tls.key" \
    --dry-run=client -o yaml | k apply -f - >/dev/null
  rm -rf "${tmp}"
}

# Fills the global HELM_ARGS array. Helm applies them in order, so later ones win:
# values files first, then the --set flags that depend on this machine.
build_helm_args() {
  local node_ip
  node_ip="$(docker inspect "${CLUSTER_NAME}-control-plane" \
    -f "{{(index .NetworkSettings.Networks \"${CLUSTER_NAME}\").IPAddress}}")"

  # A new lab starts on the host network, because there is no CNI yet.
  local host_network=true
  if [[ "$(release_on_host_network)" == false ]] || all_deployed; then
    host_network=false
  fi

  HELM_ARGS=(-f "${MANIFESTS}/values.yaml")
  if (( KAAS == 1 )); then
    HELM_ARGS+=(-f "${MANIFESTS}/kaas.yaml")
  fi
  if [[ -f "${DIR}/local.yaml" ]]; then
    HELM_ARGS+=(-f "${DIR}/local.yaml")
  fi

  HELM_ARGS+=(
    --set "installOnClusterWithoutCNI=${host_network}"
    --set "${LOCAL_APPS}.kubeovn.values.masterNodes=${node_ip}"
    # KaaS worker VMs reach Traefik on the control-plane IP: 127.0.0.1 would be the VM itself.
    --set "${MANAGEMENT_APPS}.superphenix-api.values.config.productsConfig.argoApp.kubernetes.azDomains.local.internal=${node_ip}.nip.io"
  )

  if [[ ! -e /dev/kvm ]]; then
    HELM_ARGS+=(--set "${LOCAL_APPS}.kubevirt-config.values.config.configuration.developerConfiguration.useEmulation=true")
  fi

  # Images built by `dev`: each file holds one Helm argument per line.
  local file lines
  for file in "${GENERATED}"/dev-*.args; do
    if [[ -f "${file}" ]]; then
      mapfile -t lines < "${file}"
      HELM_ARGS+=("${lines[@]}")
    fi
  done
}

# =============================================================================
# Step 4: wait for Superphenix
# =============================================================================

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
    if all_deployed; then
      ok "every cluster is Deployed"
      return 0
    fi
    echo "  $(date +%H:%M:%S)  $(progress)"
    sleep 30
  done
  return 1
}

# =============================================================================
# Commands
# =============================================================================

cmd_up() {
  step "Checking the host"
  preflight

  step "Creating the cluster"
  create_cluster

  step "Installing Superphenix"
  cmd_apply

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
  if cluster_exists; then
    kind delete cluster --name "${CLUSTER_NAME}"
  fi
  docker network rm "${CLUSTER_NAME}" >/dev/null 2>&1 || true
  rm -rf "${GENERATED}"
  ok "lab deleted (the CA in ${CA_DIR} is kept)"
}

cmd_apply() {
  cluster_exists || die "No lab yet. Run 'make up' first."

  if [[ "${SPX_VERSION}" == latest ]]; then
    SPX_VERSION="$(helm show chart "${CHART}" | awk '/^version:/ {print $2}')"
  fi

  install_certificate
  build_helm_args

  # The released chart, or the one in your checkout after `dev operator`.
  local chart=("${CHART}" --version "${SPX_VERSION}")
  if [[ -f "${GENERATED}/dev-operator.args" ]]; then
    chart=("${SUPERPHENIX_REPO}/components/system/superphenix-operator"
           --set "config.system.version=${SPX_VERSION}")
  fi

  # The chart also defines its own namespace: this label and these annotations
  # let Helm adopt the one we create here.
  k get namespace "${NAMESPACE}" >/dev/null 2>&1 || k create namespace "${NAMESPACE}" >/dev/null
  k label namespace "${NAMESPACE}" app.kubernetes.io/managed-by=Helm --overwrite >/dev/null
  k annotate namespace "${NAMESPACE}" --overwrite \
    meta.helm.sh/release-name="${RELEASE}" \
    meta.helm.sh/release-namespace="${NAMESPACE}" >/dev/null

  local output
  if ! output="$(helm --kube-context "${KUBE_CONTEXT}" upgrade --install "${RELEASE}" \
                   -n "${NAMESPACE}" "${chart[@]}" "${HELM_ARGS[@]}" 2>&1)"; then
    echo "${output}" >&2
    die "helm upgrade failed."
  fi

  if (( KAAS == 1 )); then
    ok "Superphenix ${SPX_VERSION} applied with KaaS"
  else
    ok "Superphenix ${SPX_VERSION} applied"
  fi
}

cmd_info() {
  local password
  password="$(k -n "${NAMESPACE}" get secret argocd-initial-admin-secret \
    -o jsonpath='{.data.password}' 2>/dev/null | base64 -d || true)"

  cat <<EOF

  Console   https://console.${DOMAIN}   (sign up with any email)
  ArgoCD    https://argocd.${DOMAIN}    (admin / ${password:-not ready yet})
  Grafana   https://grafana.${DOMAIN}
  kubectl   kubectl --context ${KUBE_CONTEXT}

EOF
}

cmd_status() {
  cluster_exists || die "No lab yet. Run 'make up' first."

  step "Superphenix clusters"
  k -n "${NAMESPACE}" get clusters.operator.superphenix.net || true

  step "ArgoCD applications not Synced and Healthy"
  k -n "${NAMESPACE}" get applications.argoproj.io -o json | jq -r '
    .items[]
    | select(.status.sync.status != "Synced" or .status.health.status != "Healthy")
    | [ .metadata.name,
        .status.sync.status // "-",
        .status.health.status // "-",
        (.status.conditions[0].message // .status.operationState.message // "" | .[0:100])
      ]
    | @tsv
  ' | column -t -s $'\t'

  step "Pods not ready"
  # Columns: NAMESPACE NAME READY STATUS ... Show pods not Running, or Running
  # with fewer ready containers than total (READY like 1/2). Skip finished jobs.
  k get pods -A --no-headers | awk '
    $4 != "Completed" {
      split($3, ready, "/")
      if ($4 != "Running" || ready[1] != ready[2]) print
    }'

  cmd_info
}

# Builds one component from your checkout, loads the image into the kind nodes,
# and points the Helm values at it.
cmd_dev() {
  local component="${1:-}"
  local repo="${SUPERPHENIX_REPO}" repo_setting="SUPERPHENIX_REPO"
  local dockerfile="${SUPERPHENIX_REPO}/components/system/superphenix-${component}/docker/Dockerfile"
  local values_prefix   # where the component's image.repository and image.tag live

  case "${component}" in
    operator)
      values_prefix="" ;;
    controller)
      values_prefix="${LOCAL_APPS}.superphenix-controller.values." ;;
    api)
      values_prefix="${MANAGEMENT_APPS}.superphenix-api.values." ;;
    console)
      values_prefix="${MANAGEMENT_APPS}.superphenix-console.values."
      repo="${CONSOLE_REPO}"
      repo_setting="CONSOLE_REPO"
      dockerfile="${CONSOLE_REPO}/Dockerfile" ;;
    reset)
      rm -f "${GENERATED}"/dev-*.args
      cmd_apply
      return ;;
    *)
      die "Usage: ./superphenix-kind.sh dev <operator|controller|api|console|reset>" ;;
  esac

  [[ -f "${dockerfile}" ]] || die "No ${component} checkout at ${repo}. Set ${repo_setting} (see .env.example)."

  local image="localhost/superphenix-${component}"
  local tag
  tag="dev-$(date +%Y%m%d%H%M%S)"
  docker build -t "${image}:${tag}" --build-arg "VERSION=${SPX_VERSION}" -f "${dockerfile}" "${repo}"

  # Load the image straight into each node's containerd (what `kind load` does).
  local node
  for node in $(kind get nodes --name "${CLUSTER_NAME}"); do
    docker save "${image}:${tag}" \
      | docker exec -i "${node}" ctr --namespace=k8s.io images import --digests - >/dev/null \
      || die "Could not load ${image}:${tag} on ${node}."
  done

  if [[ "${component}" == operator ]]; then
    k apply --server-side --force-conflicts \
      -f "${SUPERPHENIX_REPO}/components/system/superphenix-operator/crds" >/dev/null
  fi

  mkdir -p "${GENERATED}"
  printf '%s\n' \
    --set "${values_prefix}image.repository=${image}" \
    --set "${values_prefix}image.tag=${tag}" \
    > "${GENERATED}/dev-${component}.args"

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

# =============================================================================
# Main
# =============================================================================

case "${1:-}" in
  up)     cmd_up ;;
  down)   cmd_down ;;
  status) cmd_status ;;
  apply)  cmd_apply ;;
  dev)    cmd_dev "${2:-}" ;;
  ca)     cmd_ca ;;
  *)      usage; exit 1 ;;
esac
