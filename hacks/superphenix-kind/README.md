# Superphenix in kind

Run Superphenix end to end in
[kind](https://kind.sigs.k8s.io) on a laptop: console, networking, VMs and,
optionally, Kubernetes as a Service. It installs the released charts, like a
real install. No special hardware, no Ceph.

```bash
make up          # 20-40 min the first time
make status      # what is not healthy yet, URLs and passwords
make down        # delete everything
```

> [!WARNING]
> This is a dev lab with public passwords. Don't expose it to a network.

## Requirements

- Linux x86-64 or Windows with WSL2 (not ARM64 compatible: the released Superphenix images are based on amd64 only)
- Docker, kind v0.33.0 or newer (older versions fail on the Kubernetes 1.37 node image), kubectl, Helm, jq, openssl, `sudo` (to load kernel modules)
- 16 GiB RAM, 8 threads, ~30 GiB disk, ports 80 and 443 free
- `/dev/kvm` makes VMs fast. Without it they run emulated: slow, but they work.

On WSL2, give WSL more memory in `%UserProfile%\.wslconfig`, then run
`wsl --shutdown`:

```ini
[wsl2]
memory=24GB
processors=8
```

macOS is not supported yet. On a Mac, run
the lab on a remote x86-64 Linux machine and forward the ports over SSH, so
the URLs below work unchanged:

```bash
ssh -L 80:127.0.0.1:80 -L 443:127.0.0.1:443 you@lab-host
```

Binding ports 80 and 443 on the Mac needs `sudo ssh ...`.   
For API and console work only, the Docker Compose stack in [`../compose/`](../compose/) is a
lighter option that needs no cluster.

## Using it

| Service | URL                              | Login                                |
| ------- | -------------------------------- | ------------------------------------ |
| Console | https://console.127.0.0.1.nip.io | sign up with any email               |
| ArgoCD  | https://argocd.127.0.0.1.nip.io  | `admin`, password from `make status` |
| Grafana | https://grafana.127.0.0.1.nip.io | `admin`, secret `prometheus-grafana` |

Run `make ca` once to trust the lab's HTTPS certificate. The kubectl context
is `kind-spx-lab`.

To try it: create an organization, a project, a VPC, a subnet, a NAT gateway,
then a VM. With `make up KAAS=1`, also a Kubernetes cluster.

## Running your own code

`make dev-*` builds from the checkout this folder is in. `make dev-console`
also needs the console, which lives in its own repository. Clone it next to
this one:

```bash
cd ../../..   # the folder that holds your superphenix checkout
git clone https://github.com/super-phenix/console-ui.git
```

You should end up with:

```
superphenix/   # this repository
console-ui/
```

If your console checkout is somewhere else, set `CONSOLE_REPO` in `.env`.

Then:

```bash
make dev-controller   # build and deploy your superphenix-controller
make dev-api          # same for superphenix-api
make dev-console      # same for the console, from CONSOLE_REPO
make dev-operator     # same for the operator, with its chart and CRDs
make dev-reset        # back to the released images
```

Rerun after each change. To change chart values, put them in `local.yaml`
(git-ignored) and run `make apply`.

## Files

| File                     | What it is                                                       |
| ------------------------ | ---------------------------------------------------------------- |
| `superphenix-kind.sh`    | every command; `make` only calls it                              |
| `manifests/kind.yaml`    | the kind cluster: no CNI, dual-stack, ports 80/443 on localhost  |
| `manifests/values.yaml`  | operator values: local-path storage, lab hostnames, smaller apps |
| `manifests/kaas.yaml`    | added with `KAAS=1`                                              |
| `manifests/storage.yaml` | lets VM disks use local-path (see below)                         |

## Settings

```bash
cp .env.example .env   # then uncomment what you change
```

A variable on the command line wins over `.env`, for example `make up KAAS=1`, but only for that one command. The exception is `KAAS`: if it is set nowhere, later commands like `make apply` keep what the lab runs.

## This lab compared with a real install

|                  | This lab                         | Real install                                       |
| ---------------- | -------------------------------- | -------------------------------------------------- |
| Nodes            | Docker containers on your kernel | Talos on bare metal                                |
| Storage          | local-path, no live migration    | Rook/Ceph                                          |
| External network | Docker bridge, `eth0`            | a VLAN or BGP range, `ext0`                        |
| DNS              | nip.io and a CoreDNS rewrite     | a real domain                                      |
| TLS              | local CA                         | cert-manager and Let's Encrypt                     |
| Management       | same cluster as the AZ           | same cluster, or a separate one driving remote AZs |

Found a mistake, or something that's changed in a newer release? Please open an
issue or a pull request.

## Troubleshooting

`make status` shows unhealthy apps and pods. `make down up` starts over.