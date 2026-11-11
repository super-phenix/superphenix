FROM alpine:3.21

ARG KUBECTL_VERSION=v1.32.3
ARG HELM_VERSION=v4.3.1
ARG K9S_VERSION=v0.51.0
ARG ARGOCD_VERSION=v3.6.1
ARG VELERO_VERSION=v1.18.4
ARG TARGETARCH

RUN apk add --no-cache \
    bash \
    curl \
    jq \
    ca-certificates \
    git \
    vim \
    less \
    bind-tools \
    openssl \
    wget \
    tar \
    gzip

# Install kubectl
RUN case "${TARGETARCH}" in \
      amd64) ARCH="amd64" ;; \
      arm64) ARCH="arm64" ;; \
      *) ARCH="amd64" ;; \
    esac && \
    curl -fsSL "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${ARCH}/kubectl" -o /usr/local/bin/kubectl \
    && chmod +x /usr/local/bin/kubectl

# Install helm
RUN case "${TARGETARCH}" in \
      amd64) ARCH="amd64" ;; \
      arm64) ARCH="arm64" ;; \
      *) ARCH="amd64" ;; \
    esac && \
    curl -fsSL "https://get.helm.sh/helm-${HELM_VERSION}-linux-${ARCH}.tar.gz" \
    | tar -xz --strip-components=1 -C /usr/local/bin linux-${ARCH}/helm \
    && chmod +x /usr/local/bin/helm

# Install k9s
RUN case "${TARGETARCH}" in \
      amd64) ARCH="amd64" ;; \
      arm64) ARCH="arm64" ;; \
      *) ARCH="amd64" ;; \
    esac && \
    curl -fsSL "https://github.com/derailed/k9s/releases/download/${K9S_VERSION}/k9s_Linux_${ARCH}.tar.gz" \
    | tar -xz -C /usr/local/bin k9s \
    && chmod +x /usr/local/bin/k9s

# Install argocd
RUN case "${TARGETARCH}" in \
      amd64) ARCH="amd64" ;; \
      arm64) ARCH="arm64" ;; \
      *) ARCH="amd64" ;; \
    esac && \
    curl -fsSL "https://github.com/argoproj/argo-cd/releases/download/${ARGOCD_VERSION}/argocd-linux-${ARCH}" -o /usr/local/bin/argocd \
    && chmod +x /usr/local/bin/argocd

# Install velero
RUN case "${TARGETARCH}" in \
      amd64) ARCH="amd64" ;; \
      arm64) ARCH="arm64" ;; \
      *) ARCH="amd64" ;; \
    esac && \
    curl -fsSL "https://github.com/vmware-tanzu/velero/releases/download/${VELERO_VERSION}/velero-${VELERO_VERSION}-linux-${ARCH}.tar.gz" \
    | tar -xz --strip-components=1 -C /usr/local/bin "velero-${VELERO_VERSION}-linux-${ARCH}/velero" \
    && chmod +x /usr/local/bin/velero

# Add a Superphenix K9S skin and configuration
COPY components/dependencies/superphenix-toolbox/docker/k9s/skin.yaml /root/.config/k9s/skins/superphenix.yaml
COPY components/dependencies/superphenix-toolbox/docker/k9s/config.yaml /root/.config/k9s/config.yaml

# Entrypoint script: symlinks the operator-managed aggregated kubeconfig into
# ~/.kube/config so all tools pick it up automatically. The kubelet updates
# the mounted Secret file in place when credentials are rotated.
COPY components/dependencies/superphenix-toolbox/docker/toolbox-entrypoint.sh /usr/local/bin/toolbox-entrypoint.sh
RUN chmod +x /usr/local/bin/toolbox-entrypoint.sh

ENTRYPOINT ["/bin/bash", "/usr/local/bin/toolbox-entrypoint.sh"]
