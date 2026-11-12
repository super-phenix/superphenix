# This Dockerfile builds the VM disk images that are used as the basis of the
# Kubernetes as a Service product. The image must contain all the tools and
# binaries necessary to create a worker node.
#
# Inspiration was taken from the way Cozystack used to build their images
# https://github.com/cozystack/cozystack
#
# And from the Kubernetes documentation on how to bootstrap a node
# https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/

FROM ubuntu:26.04 AS guestfish

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get -q update > /dev/null \
 && apt-get -q -y install         \
     libguestfs-tools             \
     linux-image-generic          \
     wget                         \
     make                         \
     bash-completion              \
     > /dev/null

WORKDIR /build

FROM guestfish AS builder

# Component versions. These are inputs to the build and are normally overridden
# via `--build-arg` from a file in ./versions/ driven by the Makefile.
ARG KUBERNETES_VERSION
ARG CNI_PLUGINS_VERSION
ARG CONTAINERD_VERSION
ARG RUNC_VERSION
ARG RELEASE_VERSION
ARG CRICTL_VERSION

# Base cloud image (Ubuntu).
ARG OS_REL_NAME
ARG OS_REL
ARG OS_VER

RUN wget -O image.qcow2 https://cloud-images.ubuntu.com/minimal/releases/${OS_REL_NAME}/release-${OS_VER}/ubuntu-${OS_REL}-minimal-cloudimg-amd64.img --show-progress --output-file /dev/stdout --progress=dot:giga 2>/dev/null

RUN qemu-img resize image.qcow2 5G \
 && eval "$(guestfish --listen --network)" \
 && guestfish --remote add-drive image.qcow2 \
 && guestfish --remote run \
 && guestfish --remote mount /dev/sda1 / \
 && guestfish --remote sh "growpart /dev/sda 1 --verbose" \
 && guestfish --remote sh "resize2fs /dev/sda1" \
 && guestfish --remote sh "apt update -q && apt install -yq qemu-guest-agent cron haproxy" \
# Install containerd & runc
 && guestfish --remote sh "curl -L https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-amd64.tar.gz | tar -C /usr/local -xz" \
 && guestfish --remote sh "curl -L https://github.com/opencontainers/runc/releases/download/v${RUNC_VERSION}/runc.amd64 -o runc.amd64" \
 && guestfish --remote sh "install -m 755 runc.amd64 /usr/local/sbin/runc && rm runc.amd64" \
# Configure containerd
 && guestfish --remote sh "mkdir -p /usr/local/lib/systemd/system/" \
 && guestfish --remote sh "curl -L https://raw.githubusercontent.com/containerd/containerd/main/containerd.service -o /usr/local/lib/systemd/system/containerd.service" \
 && guestfish --remote sh "systemctl daemon-reload && systemctl enable containerd" \
 && guestfish --remote sh "mkdir -p /etc/containerd" \
 && guestfish --remote sh "/usr/local/bin/containerd config default | tee /etc/containerd/config.toml" \
 && guestfish --remote sh "sed -i '/SystemdCgroup/ s/=.*/= true/' /etc/containerd/config.toml" \
# Install crictl
 && guestfish --remote sh "curl -L https://github.com/kubernetes-sigs/cri-tools/releases/download/v${CRICTL_VERSION}/crictl-v${CRICTL_VERSION}-linux-amd64.tar.gz | tar -C /usr/local/bin -xz" \
# Install kubeadm
 && guestfish --remote sh "curl -L --remote-name-all https://dl.k8s.io/release/v${KUBERNETES_VERSION}/bin/linux/amd64/kubeadm -o /usr/bin/kubeadm" \
 && guestfish --remote sh "chmod +x /usr/bin/kubeadm" \
# Install kubelet
 && guestfish --remote sh "curl -L --remote-name-all https://dl.k8s.io/release/v${KUBERNETES_VERSION}/bin/linux/amd64/kubelet -o /usr/bin/kubelet" \
 && guestfish --remote sh "chmod +x /usr/bin/kubelet" \
 && guestfish --remote sh "curl -sSL https://raw.githubusercontent.com/kubernetes/release/v${RELEASE_VERSION}/cmd/krel/templates/latest/kubelet/kubelet.service -o /usr/lib/systemd/system/kubelet.service" \
 && guestfish --remote sh "mkdir -p /usr/lib/systemd/system/kubelet.service.d" \
 && guestfish --remote sh "curl -sSL https://raw.githubusercontent.com/kubernetes/release/v${RELEASE_VERSION}/cmd/krel/templates/latest/kubeadm/10-kubeadm.conf | tee /usr/lib/systemd/system/kubelet.service.d/10-kubeadm.conf" \
 && guestfish --remote sh "mkdir -p /etc/kubernetes/manifests" \
 && guestfish --remote sh "systemctl enable kubelet" \
# Install CNI Plugins
 && guestfish --remote sh "mkdir -p /opt/cni/bin" \
 && guestfish --remote sh "curl -L https://github.com/containernetworking/plugins/releases/download/v${CNI_PLUGINS_VERSION}/cni-plugins-linux-amd64-v${CNI_PLUGINS_VERSION}.tgz -o cni-plugins.tgz"  \
 && guestfish --remote sh "tar -C /opt/cni/bin -xzf cni-plugins.tgz" \
 && guestfish --remote sh "rm cni-plugins.tgz" \
# Clean apt cache
 && guestfish --remote sh 'apt-get clean && rm -rf /var/lib/apt/lists/*' \
# Write system configuration
 && guestfish --remote sh 'printf "%s\n" net.bridge.bridge-nf-call-iptables=1 net.bridge.bridge-nf-call-ip6tables=1 net.ipv4.ip_forward=1 net.ipv6.conf.all.forwarding=1 net.ipv6.conf.all.disable_ipv6=0 net.ipv4.tcp_congestion_control=bbr vm.overcommit_memory=1 kernel.panic=10 kernel.panic_on_oops=1 fs.inotify.max_user_instances=8192 fs.inotify.max_user_watches=524288 | tee > /etc/sysctl.d/kubernetes.conf' \
 && guestfish --remote sh 'printf "%s\n" overlay br_netfilter | tee /etc/modules-load.d/kubernetes.conf' \
 && guestfish --remote sh "systemctl enable fstrim.timer" \
# Umount all and exit
 && guestfish --remote umount-all \
 && guestfish --remote exit

FROM scratch
COPY --from=builder /build/image.qcow2 /disk/image.qcow2
