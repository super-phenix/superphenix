#!/bin/bash
set -euo pipefail

# The operator reconciles a single aggregated kubeconfig Secret
# (superphenix-toolbox-kubeconfig) that is mounted at /etc/superphenix/toolbox/kubeconfig.
# Kubernetes updates the mounted file in place whenever the Secret changes,
# so the merged kubeconfig is always up to date without any extra work.
KUBECONFIG_FILE="${KUBECONFIG_FILE:-/etc/superphenix/toolbox/kubeconfig}"
KUBE_DIR="${HOME}/.kube"
K9S_SHARE_DIR="/usr/local/share/k9s"
K9S_CONFIG_DIR="${HOME}/.config/k9s"

mkdir -p "${KUBE_DIR}" "${K9S_CONFIG_DIR}/skins"

# /root is an emptyDir, so the image cannot bake k9s config there. Seed it
# on startup when the home directory does not already have a copy.
if [[ ! -f "${K9S_CONFIG_DIR}/config.yaml" ]]; then
    cp "${K9S_SHARE_DIR}/config.yaml" "${K9S_CONFIG_DIR}/config.yaml"
fi
if [[ ! -f "${K9S_CONFIG_DIR}/skins/superphenix.yaml" ]]; then
    cp "${K9S_SHARE_DIR}/skins/superphenix.yaml" "${K9S_CONFIG_DIR}/skins/superphenix.yaml"
fi
echo "[toolbox] K9s config: ${K9S_CONFIG_DIR}"

echo "[toolbox] Superphenix toolbox starting."
echo "[toolbox] Kubeconfig source: ${KUBECONFIG_FILE}"
echo "[toolbox] The operator keeps the kubeconfig up to date as cluster credentials change."

# Wait for the kubeconfig to be available (the Secret may not be populated yet on first start).
until [[ -f "${KUBECONFIG_FILE}" ]]; do
    echo "[toolbox] Kubeconfig not yet available at ${KUBECONFIG_FILE}, retrying in 5s..."
    sleep 5
done

# Symlink the operator-managed kubeconfig into ~/.kube/config so that
# all tools (kubectl, helm, k9s, argocd, velero) pick it up without extra configuration.
ln -sf "${KUBECONFIG_FILE}" "${KUBE_DIR}/config"
echo "[toolbox] Kubeconfig linked: ${KUBECONFIG_FILE} -> ${KUBE_DIR}/config"

# Keep the container alive. The kubelet updates the mounted Secret file in
# place when credentials are rotated, so no polling loop is needed.
exec sleep infinity
