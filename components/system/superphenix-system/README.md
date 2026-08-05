# superphenix-system

![Version: 0.0.0](https://img.shields.io/badge/Version-0.0.0-informational?style=flat-square)  ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square)

Helm chart to install Superphenix system on all Superphenix clusters (management, hyperconverged, storage, workload)

This chart is the central piece of the Superphenix system installation. It is responsible for installing and configuring all the necessary components on **all Superphenix clusters**, regardless of their role:

- **Management clusters**: host the Superphenix control plane (console, API, identity, database, ...).
- **Hyperconverged clusters**: combine storage and workload on the same nodes.
- **Storage clusters**: dedicated to storage workloads (Rook / Ceph).
- **Workload clusters**: dedicated to running virtualized workloads (KubeVirt).

For more information about the different deployment topologies and cluster types, please refer to the official [Superphenix Deployment Topology documentation](https://docs.superphenix.net/architecture/deployment-topology/).

## How it works

For every entry in `.Values.apps` the chart renders a single Argo CD `Application` object. Whether an entry actually results in an `Application` depends on three inputs:

1. The global switch `.Values.disableAll` — when `true`, nothing is deployed.
2. The per-app switch `.Values.apps.<name>.enabled`.
3. The per-app `modes` list, matched against the *effective mode* of the target cluster.

The effective mode is the concatenation of `.Values.cluster.deploymentTopology` and `.Values.cluster.type`, giving one of:

| deploymentTopology | type              | Effective mode              |
| ------------------ | ----------------- | --------------------------- |
| `Hyperconverged`   | *(empty)*         | `Hyperconverged`            |
| `Decoupled`        | `Storage`         | `DecoupledStorage`          |
| `Decoupled`        | `Workload`        | `DecoupledWorkload`   |
| *(empty)*          | `Management`      | `Management`                |

- If an application's `modes` list is empty or not specified, it is considered eligible for deployment in any effective mode (as long as it is enabled).
- If a list is provided, the application is deployed only if the cluster's effective mode is present in the list.

### Global overrides

A handful of top-level values act as blast-radius controls over every generated Application:

| Field                | Effect                                                                                                     |
| -------------------- | ---------------------------------------------------------------------------------------------------------- |
| `disableAll`         | Skip rendering every Application (useful for one-off maintenance).                                         |
| `forceManual`        | Force `syncPolicy.automated.enabled = false` on every Application, regardless of per-app `automation.enabled`. |
| `cleanupOnDeletion`  | Add the `resources-finalizer.argocd.argoproj.io` finalizer on every Application, forcing cascade delete.   |

### Version parity

Applications that omit `targetRevision` (or set it to `""`) inherit `Chart.AppVersion` — the tag of this Git repository. That keeps every Superphenix-owned component in lockstep with the release of this chart.

## Values

<table>
	<thead>
		<th>Key</th>
		<th>Type</th>
		<th>Default</th>
		<th>Description</th>
	</thead>
	<tbody>
		<tr>
			<td>apps</td>
			<td>object</td>
			<td><pre lang="json">
{
  "cdi": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "cdi",
      "releaseName": "cdi"
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "cdi-system",
    "repoURL": "oci://ghcr.io/super-phenix/charts/cdi",
    "targetRevision": "0.1.0",
    "wave": "5"
  },
  "cert-manager": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "cert-manager",
      "releaseName": "cert-manager",
      "values": {
        "crds": {
          "enabled": true
        },
        "enableCertificateOwnerRef": true,
        "extraObjects": [
          "apiVersion: cert-manager.io/v1\nkind: ClusterIssuer\nmetadata:\n  name: letsencrypt\nspec:\n  acme:\n    # The ACME server URL\n    server: https://acme-v02.api.letsencrypt.org/directory\n\n    # Name of a secret used to store the ACME account private key\n    privateKeySecretRef:\n      name: letsencrypt\n\n    # Enable the HTTP-01 challenge provider\n    solvers:\n    - http01:\n        ingress:\n          class: traefik\n"
        ],
        "prometheus": {
          "servicemonitor": {
            "enabled": true
          }
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledStorage",
      "DecoupledWorkload"
    ],
    "namespace": "cert-manager-system",
    "repoURL": "https://charts.jetstack.io",
    "targetRevision": "1.19.1",
    "wave": "-10"
  },
  "cilium": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "cilium",
      "releaseName": "cilium",
      "values": {
        "cgroup": {
          "autoMount": {
            "enabled": false
          },
          "hostRoot": "/sys/fs/cgroup"
        },
        "envoy": {
          "enabled": false
        },
        "hubble": {
          "enabled": false
        },
        "ipam": {
          "mode": "kubernetes"
        },
        "ipv4": {
          "enabled": true
        },
        "ipv6": {
          "enabled": true
        },
        "k8s": {
          "requireIPv4PodCIDR": true,
          "requireIPv6PodCIDR": true
        },
        "k8sServiceHost": "localhost",
        "k8sServicePort": 7445,
        "kubeProxyReplacement": true,
        "securityContext": {
          "capabilities": {
            "ciliumAgent": [
              "CHOWN",
              "KILL",
              "NET_ADMIN",
              "NET_RAW",
              "IPC_LOCK",
              "SYS_ADMIN",
              "SYS_RESOURCE",
              "DAC_OVERRIDE",
              "FOWNER",
              "SETGID",
              "SETUID"
            ],
            "cleanCiliumState": [
              "NET_ADMIN",
              "SYS_ADMIN",
              "SYS_RESOURCE"
            ]
          }
        }
      }
    },
    "modes": [
      "DecoupledStorage"
    ],
    "namespace": "kube-system",
    "repoURL": "https://helm.cilium.io/",
    "targetRevision": "1.17.2",
    "wave": "-100"
  },
  "cluster-api-operator": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "cluster-api-operator",
      "releaseName": "cluster-api-operator",
      "values": {
        "bootstrap": {
          "kubeadm": {
            "version": "v1.12.10"
          }
        },
        "controlPlane": {
          "kamaji": {
            "version": "v0.19.0"
          }
        },
        "core": {
          "cluster-api": {
            "version": "v1.12.10"
          }
        },
        "infrastructure": {
          "kubevirt": {
            "version": "v0.11.2"
          }
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "capi-operator-system",
    "repoURL": "https://kubernetes-sigs.github.io/cluster-api-operator",
    "targetRevision": "0.28.0",
    "wave": "15"
  },
  "coredns": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": [
        "SkipDryRunOnMissingResource=true"
      ]
    },
    "enabled": true,
    "helm": {
      "chart": "coredns",
      "releaseName": "coredns",
      "values": {
        "prometheus": {
          "monitor": {
            "enabled": true
          },
          "service": {
            "enabled": true
          }
        },
        "replicaCount": 2,
        "service": {
          "clusterIP": "fd00:100:ffff::a",
          "clusterIPs": [
            "fd00:100:ffff::a",
            "10.16.0.10"
          ],
          "ipFamilyPolicy": "RequireDualStack"
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledStorage",
      "DecoupledWorkload"
    ],
    "namespace": "coredns-system",
    "repoURL": "https://coredns.github.io/helm",
    "targetRevision": "1.37.3",
    "wave": "-100"
  },
  "csi-addons": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "csi-addons",
      "releaseName": "csi-addons"
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "csi-addons-system",
    "path": ".",
    "repoURL": "oci://ghcr.io/super-phenix/charts/csi-addons",
    "targetRevision": "0.1.0",
    "wave": "0"
  },
  "csi-snapshotter": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "external-snapshotter",
      "releaseName": "external-snapshotter"
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "csi-snapshotter-system",
    "path": ".",
    "repoURL": "oci://ghcr.io/super-phenix/charts/external-snapshotter",
    "targetRevision": "0.1.0",
    "wave": "0"
  },
  "etcd-operator": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "etcd-operator",
      "releaseName": "etcd-operator",
      "values": {
        "etcdOperator": {
          "vpa": {
            "enabled": false
          }
        },
        "kubeRbacProxy": {
          "vpa": {
            "enabled": false
          }
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "etcd-operator-system",
    "repoURL": "ghcr.io/cozystack/charts",
    "targetRevision": "0.5.3",
    "wave": "5"
  },
  "gateway-api": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "directory": {
      "recurse": true
    },
    "enabled": true,
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "kube-system",
    "path": "config/crd/standard",
    "repoURL": "https://github.com/kubernetes-sigs/gateway-api.git",
    "targetRevision": "v1.6.1",
    "wave": "0"
  },
  "idrac-exporter": {
    "automation": {
      "enabled": true,
      "prune": true,
      "selfHeal": true
    },
    "enabled": false,
    "helm": {
      "chart": "idrac-exporter",
      "releaseName": "idrac-exporter",
      "values": {}
    },
    "modes": [
      "Management"
    ],
    "repoURL": "https://mrlhansen.github.io/idrac_exporter",
    "targetRevision": "2.6.1"
  },
  "ingress-nginx": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "ingress-nginx",
      "releaseName": "ingress-nginx",
      "values": {
        "controller": {
          "config": {
            "enable-real-ip": "true",
            "hsts": "false",
            "log-format-upstream": "{\"msec\": \"$msec\", \"connection\": \"$connection\", \"connection_requests\": \"$connection_requests\", \"pid\": \"$pid\", \"request_id\": \"$request_id\", \"request_length\": \"$request_length\", \"remote_addr\": \"$remote_addr\", \"remote_user\": \"$remote_user\", \"remote_port\": \"$remote_port\", \"time_local\": \"$time_local\", \"time_iso8601\": \"$time_iso8601\", \"request\": \"$request\", \"request_uri\": \"$request_uri\", \"args\": \"$args\", \"status\": \"$status\", \"body_bytes_sent\": \"$body_bytes_sent\", \"bytes_sent\": \"$bytes_sent\", \"http_referer\": \"$http_referer\", \"http_user_agent\": \"$http_user_agent\", \"http_x_forwarded_for\": \"$http_x_forwarded_for\", \"http_host\": \"$http_host\", \"server_name\": \"$server_name\", \"request_time\": \"$request_time\", \"upstream\": \"$upstream_addr\", \"upstream_connect_time\": \"$upstream_connect_time\", \"upstream_header_time\": \"$upstream_header_time\", \"upstream_response_time\": \"$upstream_response_time\", \"upstream_response_length\": \"$upstream_response_length\", \"upstream_cache_status\": \"$upstream_cache_status\", \"ssl_protocol\": \"$ssl_protocol\", \"ssl_cipher\": \"$ssl_cipher\", \"scheme\": \"$scheme\", \"request_method\": \"$request_method\", \"server_protocol\": \"$server_protocol\", \"pipe\": \"$pipe\", \"gzip_ratio\": \"$gzip_ratio\", \"http_cf_ray\": \"$http_cf_ray\"}",
            "use-forwarded-headers": "true"
          },
          "containerPort": {
            "http": 80,
            "https": 443
          },
          "hostNetwork": true,
          "ingressClass": "spx-nginx",
          "ingressClassResource": {
            "name": "spx-nginx"
          },
          "kind": "DaemonSet",
          "metrics": {
            "enabled": true,
            "serviceMonitor": {
              "additionalLabels": {
                "release": "prometheus"
              },
              "enabled": true
            }
          },
          "service": {
            "ports": {
              "http": 80,
              "https": 443
            },
            "targetPorts": {
              "http": 8080,
              "https": 8443
            },
            "type": "ClusterIP"
          },
          "tolerations": [
            {
              "effect": "NoSchedule",
              "key": "node-role.kubernetes.io/master",
              "operator": "Exists"
            }
          ]
        }
      }
    },
    "modes": [
      "DecoupledStorage"
    ],
    "namespace": "ingress-nginx-system",
    "nsLabels": {
      "pod-security.kubernetes.io/enforce": "privileged"
    },
    "repoURL": "https://kubernetes.github.io/ingress-nginx",
    "targetRevision": "4.13.3",
    "wave": "-10"
  },
  "kaas-controller": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "kaas-controller",
      "releaseName": "kaas-controller"
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "kaas-system",
    "repoURL": "ghcr.io/super-phenix/charts",
    "targetRevision": "",
    "wave": "15"
  },
  "kaas-datastore": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "kaas-datastore",
      "releaseName": "kaas-datastore",
      "values": {}
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "kaas-datastore-system",
    "repoURL": "ghcr.io/super-phenix/charts",
    "targetRevision": "",
    "wave": "5"
  },
  "kamaji": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "kamaji",
      "releaseName": "kamaji",
      "values": {
        "affinity": {
          "podAntiAffinity": {
            "preferredDuringSchedulingIgnoredDuringExecution": [
              {
                "podAffinityTerm": {
                  "labelSelector": {
                    "matchLabels": {
                      "app.kubernetes.io/name": "kamaji"
                    }
                  },
                  "topologyKey": "kubernetes.io/hostname"
                },
                "weight": 1
              }
            ]
          }
        },
        "defaultDatastoreName": "",
        "extraArgs": [
          "--certificate-expiration-deadline=336h"
        ],
        "image": {
          "tag": "26.7.3-edge"
        },
        "kamaji-etcd": {
          "deploy": false
        },
        "replicaCount": 3,
        "resources": {
          "limits": {
            "cpu": "1000m",
            "memory": "2Gi"
          },
          "requests": {
            "cpu": "100m",
            "memory": "200Mi"
          }
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "kamaji-system",
    "repoURL": "harbor.agc.dpk-agc-cl04.agoracalyce.net/spx-helm",
    "targetRevision": "26.7.22",
    "wave": "10"
  },
  "kratos": {
    "automation": {
      "enabled": true,
      "prune": true,
      "selfHeal": true
    },
    "enabled": true,
    "helm": {
      "chart": "kratos",
      "releaseName": "kratos",
      "values": {
        "ingress": {
          "public": {
            "annotations": {},
            "className": "",
            "enabled": true,
            "hosts": [
              {
                "host": "{{ (urlParse $.Values.apps.kratos.helm.values.kratos.config.serve.public.base_url).host }}",
                "paths": [
                  {
                    "path": "/",
                    "pathType": "ImplementationSpecific"
                  }
                ]
              }
            ],
            "tls": [
              {
                "hosts": [
                  "{{ (urlParse $.Values.apps.kratos.helm.values.kratos.config.serve.public.base_url).host }}"
                ],
                "secretName": "kratos-public-tls"
              }
            ]
          }
        },
        "kratos": {
          "automigration": {
            "enabled": true
          },
          "config": {
            "cookies": {
              "domain": "{{ (urlParse $.Values.apps.kratos.helm.values.kratos.config.serve.public.base_url).host }}",
              "same_site": "Strict"
            },
            "dsn": "postgres://superphenix:{{ $.Values.apps.postgres.helm.values.auth.password }}@postgres.{{ $.Release.Namespace }}.svc:5432/kratos?sslmode=disable",
            "identity": {
              "default_schema_id": "default",
              "schemas": [
                {
                  "id": "default",
                  "url": "file:///etc/config/identity.default.schema.json"
                }
              ]
            },
            "secrets": {
              "default": [
                "base64=="
              ]
            },
            "selfservice": {
              "allowed_return_urls": [
                "https://*.{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/"
              ],
              "default_browser_return_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/",
              "flows": {
                "error": {
                  "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/error"
                },
                "login": {
                  "after": {
                    "hooks": [
                      {
                        "hook": "require_verified_address"
                      }
                    ]
                  },
                  "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/login"
                },
                "recovery": {
                  "after": {
                    "hooks": [
                      {
                        "hook": "revoke_active_sessions"
                      }
                    ]
                  },
                  "enabled": true,
                  "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/recovery"
                },
                "registration": {
                  "after": {
                    "code": {
                      "hooks": [
                        {
                          "hook": "show_verification_ui"
                        }
                      ]
                    },
                    "password": {
                      "hooks": [
                        {
                          "hook": "show_verification_ui"
                        }
                      ]
                    },
                    "webauthn": {
                      "hooks": [
                        {
                          "hook": "show_verification_ui"
                        }
                      ]
                    }
                  },
                  "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/registration"
                },
                "settings": {
                  "privileged_session_max_age": "15m",
                  "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/settings"
                },
                "verification": {
                  "enabled": true,
                  "lifespan": "1h",
                  "notify_unknown_recipients": false,
                  "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/verification",
                  "use": "code"
                }
              },
              "methods": {
                "code": {
                  "config": {
                    "lifespan": "15m"
                  },
                  "enabled": true
                }
              }
            },
            "serve": {
              "public": {
                "base_url": "https://auth.console.superphenix.net",
                "cors": {
                  "allow_credentials": true,
                  "allowed_headers": [
                    "Authorization",
                    "Cookie",
                    "Content-Type",
                    "Max-Age",
                    "X-Session-Token",
                    "X-XSRF-TOKEN",
                    "X-CSRF-TOKEN"
                  ],
                  "allowed_methods": [
                    "POST",
                    "GET",
                    "PUT",
                    "PATCH",
                    "DELETE"
                  ],
                  "allowed_origins": [
                    "https://*.{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}",
                    "https://*.{{ (index $.Values.apps \"superphenix-api\").helm.values.domain }}"
                  ],
                  "enabled": true,
                  "exposed_headers": [
                    "Content-Type",
                    "Set-Cookie"
                  ]
                },
                "request_log": {
                  "disable_for_health": true
                }
              }
            }
          },
          "identitySchemas": {
            "identity.default.schema.json": "{\n  \"$id\": \"https://schemas.ory.sh/presets/kratos/identity.email.schema.json\",\n  \"$schema\": \"http://json-schema.org/draft-07/schema#\",\n  \"title\": \"Person\",\n  \"type\": \"object\",\n  \"properties\": {\n    \"traits\": {\n      \"type\": \"object\",\n\n      \"properties\": {\n        \"email\": {\n          \"type\": \"string\",\n          \"format\": \"email\",\n          \"title\": \"Email\",\n          \"ory.sh/kratos\": {\n            \"credentials\": {\n              \"password\": {\n                \"identifier\": true\n              }\n            },\n            \"recovery\": {\n              \"via\": \"email\"\n            },\n            \"verification\": {\n              \"via\": \"email\"\n            }\n          }\n        },\n        \"name\": {\n          \"type\": \"object\",\n          \"properties\": {\n            \"first\": {\n              \"type\": \"string\",\n              \"title\": \"First name\"\n            },\n            \"last\": {\n              \"type\": \"string\",\n              \"title\": \"Last name\"\n            }\n          }\n        }\n      },\n      \"required\": [\"email\"],\n      \"additionalProperties\": false\n    }\n  }\n}\n"
          }
        }
      }
    },
    "modes": [
      "Management"
    ],
    "repoURL": "https://k8s.ory.sh/helm/charts",
    "targetRevision": "0.43.1"
  },
  "kubeovn": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "kube-ovn-v2",
      "releaseName": "kube-ovn",
      "values": {
        "apiNad": {
          "enabled": true
        },
        "extraObjects": [
          {
            "apiVersion": "kubeovn.io/v1",
            "kind": "Subnet",
            "metadata": {
              "name": "system-blackhole"
            },
            "spec": {
              "acls": [
                {
                  "action": "drop",
                  "direction": "from-lport",
                  "match": "ip"
                },
                {
                  "action": "drop",
                  "direction": "to-lport",
                  "match": "ip"
                }
              ],
              "cidrBlock": "fd00:666::/64",
              "disableGatewayCheck": true,
              "protocol": "IPv6",
              "provider": "system-blackhole.kube-system.ovn",
              "vpc": "system-blackhole-vpc"
            }
          },
          {
            "apiVersion": "kubeovn.io/v1",
            "kind": "Vpc",
            "metadata": {
              "name": "system-blackhole-vpc"
            },
            "spec": {}
          },
          {
            "apiVersion": "k8s.cni.cncf.io/v1",
            "kind": "NetworkAttachmentDefinition",
            "metadata": {
              "name": "system-blackhole",
              "namespace": "kube-system"
            },
            "spec": {
              "config": "{ \"cniVersion\": \"0.3.0\", \"type\": \"kube-ovn\", \"server_socket\": \"/run/openvswitch/kube-ovn-daemon.sock\", \"provider\": \"system-blackhole.kube-system.ovn\" }"
            }
          },
          {
            "apiVersion": "kubeovn.io/v1",
            "kind": "Subnet",
            "metadata": {
              "name": "system-isolated-egress"
            },
            "spec": {
              "acls": [
                {
                  "action": "allow-related",
                  "direction": "from-lport",
                  "match": "ip4.dst == 10.16.0.10 || ip6.dst == fd00:100:ffff::a",
                  "priority": 1001
                },
                {
                  "action": "allow-related",
                  "direction": "from-lport",
                  "match": "ip4.src == 10.32.0.1 || ip6.src == fd00:110::1",
                  "priority": 1001
                },
                {
                  "action": "drop",
                  "direction": "from-lport",
                  "match": "ip4.dst == 10.0.0.0/8 \u0026\u0026 ip4.dst != 10.32.0.1 \u0026\u0026 ip4.src == 10.0.0.0/8",
                  "priority": 1000
                },
                {
                  "action": "drop",
                  "direction": "from-lport",
                  "match": "ip4.dst == 172.16.0.0/12 \u0026\u0026 ip",
                  "priority": 1000
                },
                {
                  "action": "drop",
                  "direction": "from-lport",
                  "match": "ip4.dst == 192.168.0.0/16 \u0026\u0026 ip",
                  "priority": 1000
                },
                {
                  "action": "drop",
                  "direction": "from-lport",
                  "match": "ip4.dst == 100.64.0.0/10 \u0026\u0026 ip",
                  "priority": 1000
                },
                {
                  "action": "drop",
                  "direction": "from-lport",
                  "match": "ip4.dst == 169.254.0.0/16 \u0026\u0026 ip",
                  "priority": 1000
                },
                {
                  "action": "drop",
                  "direction": "from-lport",
                  "match": "ip6.dst == fc00::/7 \u0026\u0026 ip6.dst != fd00:110::1 \u0026\u0026 ip6.src == fd00:110::/64",
                  "priority": 1000
                }
              ],
              "allowEWTraffic": false,
              "cidrBlock": "10.32.0.0/16,fd00:110::/64",
              "gatewayType": "distributed",
              "mtu": 1380,
              "natOutgoing": true,
              "protocol": "Dual",
              "provider": "system-isolated-egress.kube-system.ovn",
              "vpc": "ovn-cluster"
            }
          },
          {
            "apiVersion": "k8s.cni.cncf.io/v1",
            "kind": "NetworkAttachmentDefinition",
            "metadata": {
              "name": "system-isolated-egress",
              "namespace": "kube-system"
            },
            "spec": {
              "config": "{ \"cniVersion\": \"0.3.0\", \"type\": \"kube-ovn\", \"server_socket\": \"/run/openvswitch/kube-ovn-daemon.sock\", \"provider\": \"system-isolated-egress.kube-system.ovn\" }"
            }
          }
        ],
        "features": {
          "enableNetworkPolicies": true
        },
        "masterNodes": "invalid,invalid,invalid",
        "masterNodesLabels": {
          "kube-ovn/role": null,
          "node-role.kubernetes.io/control-plane": ""
        },
        "natGw": {
          "namePrefix": "nat-gateway"
        },
        "networking": {
          "join": {
            "cidr": {
              "v4": "100.64.0.0/12",
              "v6": "fd00:100:64::/112"
            }
          },
          "pods": {
            "cidr": {
              "v4": "10.0.0.0/12",
              "v6": "fd00:100:0000:0::/96"
            },
            "gateways": {
              "v4": "10.0.0.1",
              "v6": "fd00:100:0000:0::1"
            }
          },
          "services": {
            "cidr": {
              "v4": "10.16.0.0/12",
              "v6": "fd00:100:ffff:0::/112"
            }
          },
          "stack": "Dual"
        },
        "ovsOvn": {
          "disableModulesManagement": true,
          "ovnDirectory": "/var/lib/ovn",
          "ovsDirectory": "/var/lib/openvswitch"
        },
        "validatingWebhook": {
          "enabled": true
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "kube-system",
    "repoURL": "oci://ghcr.io/kubeovn/charts/kube-ovn-v2",
    "targetRevision": "v1.16.0",
    "wave": "-100"
  },
  "kubevirt": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "directory": {
      "recurse": true
    },
    "enabled": true,
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "kubevirt-system",
    "path": "v1.7.3/",
    "repoURL": "https://github.com/super-phenix/kubevirt-manifests.git",
    "targetRevision": "HEAD",
    "wave": "5"
  },
  "kyverno": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "kyverno",
      "releaseName": "kyverno",
      "values": {
        "reportsController": {
          "enabled": false
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "kyverno-system",
    "repoURL": "https://kyverno.github.io/kyverno/",
    "targetRevision": "3.6.0",
    "wave": "0"
  },
  "loki": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "loki",
      "releaseName": "loki",
      "values": {
        "backend": {
          "replicas": 0
        },
        "bloomCompactor": {
          "replicas": 0
        },
        "bloomGateway": {
          "replicas": 0
        },
        "chunksCache": {
          "enabled": false
        },
        "compactor": {
          "replicas": 0
        },
        "deploymentMode": "SingleBinary",
        "distributor": {
          "replicas": 0
        },
        "gateway": {
          "enabled": false
        },
        "global": {
          "clusterDomain": "",
          "dnsNamespace": "kube-system",
          "dnsService": "coredns"
        },
        "indexGateway": {
          "replicas": 0
        },
        "ingester": {
          "replicas": 0
        },
        "loki": {
          "auth_enabled": true,
          "commonConfig": {
            "replication_factor": 1
          },
          "compactor": {
            "compaction_interval": "10m",
            "delete_request_store": "filesystem",
            "retention_delete_delay": "2h",
            "retention_delete_worker_count": 150,
            "retention_enabled": true
          },
          "limits_config": {
            "retention_period": "14d"
          },
          "schemaConfig": {
            "configs": [
              {
                "from": "2024-12-01",
                "index": {
                  "period": "24h",
                  "prefix": "loki_index_"
                },
                "object_store": "filesystem",
                "schema": "v13",
                "store": "tsdb"
              }
            ]
          },
          "storage": {
            "type": "filesystem"
          }
        },
        "lokiCanary": {
          "enabled": false
        },
        "minio": {
          "enabled": false
        },
        "monitoring": {
          "serviceMonitor": {
            "enabled": true
          }
        },
        "querier": {
          "replicas": 0
        },
        "queryFrontend": {
          "replicas": 0
        },
        "queryScheduler": {
          "replicas": 0
        },
        "read": {
          "replicas": 0
        },
        "resultsCache": {
          "enabled": false
        },
        "singleBinary": {
          "persistence": {
            "enableStatefulSetAutoDeletePVC": false,
            "size": "50Gi"
          },
          "replicas": 1
        },
        "test": {
          "enabled": false
        },
        "write": {
          "replicas": 0
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledStorage",
      "DecoupledWorkload"
    ],
    "namespace": "loki-system",
    "repoURL": "https://grafana.github.io/helm-charts",
    "targetRevision": "6.43.0",
    "wave": "-5"
  },
  "metrics-server": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "metrics-server",
      "releaseName": "metrics-server",
      "values": {
        "defaultArgs": [
          "--cert-dir=/tmp",
          "--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname",
          "--kubelet-use-node-status-port",
          "--kubelet-insecure-tls",
          "--metric-resolution=15s"
        ],
        "metrics": {
          "enabled": true
        },
        "replicas": 3,
        "serviceMonitor": {
          "enabled": true
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledStorage",
      "DecoupledWorkload"
    ],
    "namespace": "metrics-server-system",
    "repoURL": "https://kubernetes-sigs.github.io/metrics-server/",
    "targetRevision": "3.13.0",
    "wave": "-10"
  },
  "multus": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "multus",
      "releaseName": "multus",
      "values": {
        "tolerations": [
          {
            "effect": "NoSchedule",
            "operator": "Exists"
          },
          {
            "effect": "NoExecute",
            "operator": "Exists"
          }
        ]
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "kube-system",
    "repoURL": "oci://ghcr.io/super-phenix/charts/multus",
    "targetRevision": "0.1.0",
    "wave": "-100"
  },
  "permify": {
    "automation": {
      "enabled": true,
      "prune": true,
      "selfHeal": true
    },
    "enabled": true,
    "helm": {
      "chart": "permify",
      "releaseName": "permify",
      "values": {
        "app": {
          "database": {
            "engine": "postgres",
            "garbage_collection": {
              "enabled": false
            },
            "uri": "postgres://superphenix:{{ (index $.Values.apps \"postgres\").helm.values.auth.password }}@postgres.{{ $.Release.Namespace }}:5432/permify"
          },
          "distributed": {
            "address": "permify.{{ $.Release.Namespace }}.svc:5000",
            "enabled": false,
            "port": 5000
          }
        }
      }
    },
    "modes": [
      "Management"
    ],
    "repoURL": "https://permify.github.io/helm-charts",
    "targetRevision": "0.3.*"
  },
  "policies": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "spx-policies",
      "releaseName": "spx-policies"
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "kube-system",
    "repoURL": "oci://ghcr.io/super-phenix/charts/spx-policies",
    "targetRevision": "0.1.0"
  },
  "postgres": {
    "automation": {
      "enabled": true,
      "prune": true,
      "selfHeal": true
    },
    "enabled": true,
    "helm": {
      "chart": "postgres",
      "releaseName": "postgres",
      "values": {
        "auth": {
          "database": "superphenix",
          "password": "changeme!",
          "username": "superphenix"
        },
        "initdb": {
          "scripts": {
            "setup.sql": "CREATE DATABASE kratos;\nCREATE DATABASE permify;\nGRANT ALL PRIVILEGES ON DATABASE kratos TO superphenix;\nGRANT ALL PRIVILEGES ON DATABASE permify TO superphenix;\n"
          }
        },
        "persistence": {
          "size": "8Gi"
        }
      }
    },
    "modes": [
      "Management"
    ],
    "repoURL": "oci://registry-1.docker.io/cloudpirates/postgres",
    "targetRevision": "0.19.12"
  },
  "prometheus-stack": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": [
        "ServerSideApply=true"
      ]
    },
    "enabled": true,
    "helm": {
      "chart": "kube-prometheus-stack",
      "releaseName": "prometheus",
      "values": {
        "alertmanager": {
          "enabled": false
        },
        "defaultRules": {
          "create": false
        },
        "grafana": {
          "additionalDataSources": [
            {
              "editable": false,
              "jsonData": {
                "httpHeaderName1": "X-Scope-OrgID"
              },
              "name": "Loki",
              "secureJsonData": {
                "httpHeaderValue1": "1"
              },
              "type": "loki",
              "url": "http://loki.loki-system:3100"
            }
          ],
          "adminPassword": "",
          "ingress": {
            "annotations": {
              "cert-manager.io/cluster-issuer": "letsencrypt"
            },
            "enabled": true,
            "hosts": [
              "invalid"
            ],
            "tls": [
              {
                "hosts": [
                  "invalid"
                ],
                "secretName": "grafana-tls"
              }
            ]
          }
        },
        "kubeControllerManager": {
          "service": {
            "selector": {
              "k8s-app": "kube-controller-manager"
            }
          }
        },
        "kubeEtcd": {
          "service": {
            "selector": {
              "k8s-app": "kube-controller-manager"
            }
          },
          "serviceMonitor": {
            "metricRelabelings": [
              {
                "action": "labeldrop",
                "regex": "pod"
              }
            ],
            "relabelings": [
              {
                "action": "replace",
                "regex": "^(.*)$",
                "replacement": "$1",
                "separator": ";",
                "sourceLabels": [
                  "__meta_kubernetes_pod_node_name"
                ],
                "targetLabel": "nodename"
              }
            ]
          }
        },
        "kubeScheduler": {
          "service": {
            "selector": {
              "k8s-app": "kube-scheduler"
            }
          }
        },
        "prometheus": {
          "prometheusSpec": {
            "podMonitorSelectorNilUsesHelmValues": false,
            "retention": "30d",
            "retentionSize": "45GB",
            "serviceMonitorSelectorNilUsesHelmValues": false,
            "storageSpec": {
              "volumeClaimTemplate": {
                "spec": {
                  "resources": {
                    "requests": {
                      "storage": "50Gi"
                    }
                  }
                }
              }
            }
          }
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledStorage",
      "DecoupledWorkload"
    ],
    "namespace": "prometheus-system",
    "nsLabels": {
      "pod-security.kubernetes.io/enforce": "privileged"
    },
    "repoURL": "https://prometheus-community.github.io/helm-charts",
    "targetRevision": "78.4.0",
    "wave": "-5"
  },
  "promtail": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "promtail",
      "releaseName": "promtail",
      "values": {
        "config": {
          "clients": [
            {
              "tenant_id": 1,
              "url": "http://loki.loki-system:3100/loki/api/v1/push"
            }
          ]
        },
        "serviceMonitor": {
          "enabled": true
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledStorage",
      "DecoupledWorkload"
    ],
    "namespace": "promtail-system",
    "nsLabels": {
      "pod-security.kubernetes.io/enforce": "privileged"
    },
    "repoURL": "https://grafana.github.io/helm-charts",
    "targetRevision": "6.17.0",
    "wave": "0"
  },
  "rook-connection": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": [
        "RespectIgnoreDifferences=true"
      ]
    },
    "enabled": true,
    "helm": {
      "chart": "spx-rook-connection",
      "releaseName": "spx-rook-connection"
    },
    "ignoreDifferences": [
      {
        "jsonPointers": [
          "/data/data",
          "/data/mapping"
        ],
        "kind": "ConfigMap"
      }
    ],
    "modes": [
      "DecoupledWorkload"
    ],
    "namespace": "rook-system",
    "nsLabels": {
      "pod-security.kubernetes.io/enforce": "privileged"
    },
    "path": ".",
    "repoURL": "oci://ghcr.io/super-phenix/charts/spx-rook-connection",
    "targetRevision": "0.2.0",
    "wave": "0"
  },
  "rook-local-cluster": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "rook-ceph-cluster",
      "releaseName": "rook-local-cluster",
      "values": {
        "cephBlockPools": [
          {
            "name": "mgr",
            "spec": {
              "deviceClass": "nvme",
              "enableCrushUpdates": true,
              "failureDomain": "host",
              "mirroring": {
                "enabled": false
              },
              "name": ".mgr",
              "parameters": {
                "compression_mode": "none"
              },
              "replicated": {
                "requireSafeReplicaSize": true,
                "size": 3
              }
            },
            "storageClass": {
              "enabled": false
            }
          },
          {
            "name": "spx-scratchpad",
            "spec": {
              "deviceClass": "nvme",
              "enableCrushUpdates": true,
              "enableRBDStats": true,
              "failureDomain": "host",
              "replicated": {
                "size": 2
              }
            },
            "storageClass": {
              "allowVolumeExpansion": true,
              "enabled": true,
              "isDefault": true,
              "mountOptions": [
                "discard"
              ],
              "name": "spx-scratchpad",
              "parameters": {
                "csi.storage.k8s.io/controller-expand-secret-name": "rook-csi-rbd-provisioner",
                "csi.storage.k8s.io/controller-expand-secret-namespace": "spx-storage",
                "csi.storage.k8s.io/fstype": "ext4",
                "csi.storage.k8s.io/node-stage-secret-name": "rook-csi-rbd-node",
                "csi.storage.k8s.io/node-stage-secret-namespace": "spx-storage",
                "csi.storage.k8s.io/provisioner-secret-name": "rook-csi-rbd-provisioner",
                "csi.storage.k8s.io/provisioner-secret-namespace": "spx-storage",
                "imageFeatures": "layering,fast-diff,object-map,deep-flatten,exclusive-lock",
                "imageFormat": "2"
              },
              "reclaimPolicy": "Delete",
              "volumeBindingMode": "Immediate"
            }
          }
        ],
        "cephClusterSpec": {
          "cephConfig": {
            "global": {
              "rbd_mirroring_delete_delay": "604800",
              "rbd_mirroring_max_mirroring_snapshots": "30",
              "rbd_move_to_trash_on_remove": "true",
              "rbd_move_to_trash_on_remove_expire_seconds": "604800"
            }
          },
          "crashCollector": {
            "daysToRetain": 365,
            "disable": false
          },
          "dashboard": {
            "enabled": true,
            "prometheusEndpoint": "http://prometheus-kube-prometheus-prometheus.prometheus-system:9090",
            "prometheusEndpointSSLVerify": false,
            "ssl": false
          },
          "mgr": {
            "modules": [
              {
                "enabled": true,
                "name": "rook"
              }
            ]
          },
          "network": {
            "addressRanges": {
              "cluster": [
                "invalid"
              ],
              "public": [
                "invalid"
              ]
            },
            "ipFamily": "IPv6",
            "provider": "host"
          },
          "storage": {
            "deviceFilter": "",
            "useAllDevices": false
          }
        },
        "cephFileSystems": [],
        "cephObjectStores": [],
        "clusterName": "invalid",
        "ingress": {
          "dashboard": {
            "annotations": {
              "cert-manager.io/cluster-issuer": "letsencrypt"
            },
            "host": {
              "name": "invalid",
              "path": "/"
            },
            "tls": [
              {
                "hosts": [
                  "invalid"
                ],
                "secretName": "ceph-dashboard-tls"
              }
            ]
          }
        },
        "monitoring": {
          "enabled": true
        },
        "operatorNamespace": "rook-system",
        "toolbox": {
          "enabled": true
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledStorage"
    ],
    "namespace": "spx-storage",
    "nsLabels": {
      "pod-security.kubernetes.io/enforce": "privileged"
    },
    "repoURL": "https://charts.rook.io/release",
    "targetRevision": "1.16.3",
    "wave": "0"
  },
  "rook-operator": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "rook-ceph",
      "releaseName": "rook-operator",
      "values": {
        "csi": {
          "csiAddons": {
            "enabled": true
          },
          "csiAddonsRBDProvisionerPort": 9071,
          "enableCephfsDriver": false,
          "enableOMAPGenerator": true,
          "serviceMonitor": {
            "enabled": true
          }
        },
        "discoveryDaemonInterval": "5m",
        "enableDiscoveryDaemon": true,
        "enforceHostNetwork": true,
        "monitoring": {
          "enabled": true
        },
        "obcAllowAdditionalConfigFields": "maxObjects,maxSize,bucketMaxObjects,bucketMaxSize,bucketPolicy,bucketLifecycle",
        "useOperatorHostNetwork": true
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledStorage",
      "DecoupledWorkload"
    ],
    "namespace": "rook-system",
    "nsLabels": {
      "pod-security.kubernetes.io/enforce": "privileged"
    },
    "repoURL": "https://charts.rook.io/release",
    "targetRevision": "1.16.5",
    "wave": "-5"
  },
  "snapscheduler": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "snapscheduler",
      "releaseName": "snapscheduler",
      "values": {
        "enableOwnerReferences": true,
        "replicaCount": 2
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "snapscheduler-system",
    "repoURL": "https://backube.github.io/helm-charts/",
    "targetRevision": "*",
    "wave": "0"
  },
  "superphenix-api": {
    "automation": {
      "enabled": true,
      "prune": true,
      "selfHeal": true
    },
    "enabled": true,
    "helm": {
      "chart": "superphenix-api",
      "releaseName": "superphenix-api",
      "values": {
        "config": {
          "database": {
            "database": "superphenix",
            "host": "postgres.{{ $.Release.Namespace }}.svc",
            "password": "{{ $.Values.apps.postgres.helm.values.auth.password | quote }}",
            "port": 5432,
            "username": "superphenix"
          },
          "permify": {
            "url": "permify.{{ $.Release.Namespace }}.svc:3478"
          }
        },
        "domain": "api.superphenix.net"
      }
    },
    "modes": [
      "Management"
    ],
    "repoURL": "oci://ghcr.io/super-phenix/charts/superphenix-api",
    "targetRevision": ""
  },
  "superphenix-console": {
    "automation": {
      "enabled": true,
      "prune": true,
      "selfHeal": true
    },
    "enabled": true,
    "helm": {
      "chart": "superphenix-console",
      "releaseName": "superphenix-console",
      "values": {
        "domain": "console.superphenix.net",
        "ingress": {
          "enabled": true,
          "hosts": [
            {
              "host": "console.superphenix.net",
              "paths": [
                {
                  "path": "/"
                }
              ]
            }
          ],
          "tls": [
            {
              "hosts": [
                "console.superphenix.net"
              ],
              "secretName": "superphenix-console-tls"
            }
          ]
        }
      }
    },
    "modes": [
      "Management"
    ],
    "repoURL": "oci://ghcr.io/super-phenix/charts/superphenix-console",
    "targetRevision": "0.2.1"
  },
  "superphenix-controller": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": [
        "SkipDryRunOnMissingResource=true"
      ]
    },
    "enabled": true,
    "helm": {
      "chart": "superphenix-controller",
      "releaseName": "superphenix-controller",
      "values": {}
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "superphenix-system",
    "repoURL": "ghcr.io/super-phenix/charts",
    "targetRevision": "",
    "wave": "-10"
  },
  "talos-backup": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "talos-backup",
      "releaseName": "talos-backup"
    },
    "modes": [
      "Hyperconverged",
      "DecoupledStorage",
      "DecoupledWorkload"
    ],
    "namespace": "talos-backup",
    "repoURL": "oci://ghcr.io/super-phenix/charts/talos-backup",
    "targetRevision": "0.1.0"
  },
  "talos-operator": {
    "automation": {
      "enabled": true,
      "prune": true,
      "selfHeal": true
    },
    "enabled": false,
    "helm": {
      "chart": "talos-operator",
      "releaseName": "talos-operator",
      "values": {
        "featureFlags": {
          "enablePxeBootStack": true
        }
      }
    },
    "modes": [
      "Management"
    ],
    "repoURL": "https://alperencelik.github.io/helm-charts",
    "targetRevision": "0.6.1"
  },
  "traefik": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "traefik",
      "releaseName": "traefik",
      "values": {
        "accessLog": {
          "enabled": true
        },
        "api": {
          "dashboard": false
        },
        "deployment": {
          "enabled": true,
          "kind": "DaemonSet"
        },
        "gateway": {
          "enabled": false
        },
        "gatewayClass": {
          "enabled": true,
          "name": "traefik"
        },
        "global": {
          "checkNewVersion": false
        },
        "hostNetwork": true,
        "ingressClass": {
          "enabled": true
        },
        "podSecurityContext": {
          "runAsGroup": 0,
          "runAsNonRoot": false,
          "runAsUser": 0
        },
        "ports": {
          "kaas-api-tls": {
            "expose": {
              "default": true
            },
            "exposedPort": 7444,
            "http": {
              "tls": {
                "enabled": true
              }
            },
            "port": 7444,
            "protocol": "TCP"
          },
          "kaas-https": {
            "expose": {
              "default": true
            },
            "exposedPort": 7443,
            "http": {
              "tls": {
                "enabled": true
              }
            },
            "port": 7443,
            "protocol": "TCP"
          },
          "kaas-konnectivity": {
            "expose": {
              "default": true
            },
            "exposedPort": 7442,
            "http": {
              "tls": {
                "enabled": true
              }
            },
            "port": 7442,
            "protocol": "TCP"
          },
          "metrics": {
            "exposedPort": 9101,
            "port": 9101
          },
          "web": {
            "port": 80
          },
          "websecure": {
            "port": 443
          }
        },
        "providers": {
          "kubernetesGateway": {
            "enabled": true
          },
          "kubernetesIngressNGINX": {
            "enabled": true,
            "watchIngressWithoutClass": true
          }
        },
        "securityContext": {
          "capabilities": {
            "add": [
              "NET_BIND_SERVICE"
            ],
            "drop": [
              "ALL"
            ]
          }
        },
        "service": {
          "type": "ClusterIP"
        },
        "updateStrategy": {
          "rollingUpdate": {
            "maxSurge": 0,
            "maxUnavailable": 1
          },
          "type": "RollingUpdate"
        }
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "traefik-system",
    "nsLabels": {
      "pod-security.kubernetes.io/enforce": "privileged"
    },
    "repoURL": "https://traefik.github.io/charts",
    "targetRevision": "41.0.1",
    "wave": "0"
  },
  "tuned": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": false,
    "helm": {
      "chart": "tuned",
      "releaseName": "tuned"
    },
    "modes": [
      "Hyperconverged",
      "DecoupledStorage",
      "DecoupledWorkload"
    ],
    "namespace": "tuned-system",
    "nsLabels": {
      "pod-security.kubernetes.io/enforce": "privileged"
    },
    "repoURL": "oci://ghcr.io/super-phenix/charts/tuned",
    "targetRevision": "0.1.0",
    "wave": "0"
  },
  "velero": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "velero",
      "releaseName": "velero",
      "values": {
        "configuration": {
          "backupStorageLocation": [],
          "defaultItemOperationTimeout": "8h",
          "features": "EnableCSI",
          "namespace": "velero-system",
          "restoreResourcePriorities": "vpc.kubeovn.io,subnet.kubeovn.io,vpc-nat-gateway.kubeovn.io,iptables-eip.kubeovn.io,iptables-fip-rule.kubeovn.io,iptables-snat-rule.kubeovn.io,iptables-dnat-rule.kubeovn.io,switch-lb-rule.kubeovn.io,networkpolicy.networking.k8s.io,network-attachment-definition.k8s.cni.cncf.io,controllerrevision.apps,datauploads.velero.io,persistentvolume,persistentvolumeclaim,datavolume.cdi.kubevirt.io,secret,virtualmachine.kubevirt.io,cluster.cluster.x-k8s.io,kubevirtcluster.infrastructure.cluster.x-k8s.io,configmap,issuer.cert-manager.io,certificate.cert-manager.io,serviceaccount,role.rbac.authorization.k8s.io,rolebinding.rbac.authorization.k8s.io,clusterrole.rbac.authorization.k8s.io,clusterrolebinding.rbac.authorization.k8s.io,etcdmember.etcd-operator.cozystack.io,etcdcluster.etcd-operator.cozystack.io,etcdcluster.etcd.aenix.io,datastore.kamaji.clastix.io,gateway.gateway.networking.k8s.io,tlsroute.gateway.networking.k8s.io,deployment.apps,kamajicontrolplane.controlplane.cluster.x-k8s.io,kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io,kubevirtmachinetemplate.infrastructure.cluster.x-k8s.io,kubeadmconfig.bootstrap.cluster.x-k8s.io,kubevirtmachine.infrastructure.cluster.x-k8s.io,machine.cluster.x-k8s.io,machineset.cluster.x-k8s.io,machinedeployment.cluster.x-k8s.io,mutatingadmissionpolicy.admissionregistration.k8s.io,mutatingadmissionpolicybinding.admissionregistration.k8s.io"
        },
        "credentials": {
          "secretContents": {
            "cloud": ""
          }
        },
        "deployNodeAgent": true,
        "extraObjects": [
          {
            "apiVersion": "v1",
            "data": {
              "node-agent-config.json": "{\n    \"loadConcurrency\": {\n        \"globalConfig\": 40\n    }\n}\n"
            },
            "kind": "ConfigMap",
            "metadata": {
              "name": "node-agent-config",
              "namespace": "velero-system"
            }
          }
        ],
        "initContainers": [
          {
            "image": "velero/velero-plugin-for-aws:v1.12.1",
            "imagePullPolicy": "IfNotPresent",
            "name": "velero-plugin-for-aws",
            "volumeMounts": [
              {
                "mountPath": "/target",
                "name": "plugins"
              }
            ]
          },
          {
            "image": "quay.io/kubevirt/kubevirt-velero-plugin:v0.8.0",
            "imagePullPolicy": "IfNotPresent",
            "name": "velero-plugin-for-kubevirt",
            "volumeMounts": [
              {
                "mountPath": "/target",
                "name": "plugins"
              }
            ]
          },
          {
            "image": "ghcr.io/super-phenix/superphenix-velero-plugin:v0.1.0",
            "imagePullPolicy": "Always",
            "name": "velero-plugin-for-superphenix",
            "volumeMounts": [
              {
                "mountPath": "/target",
                "name": "plugins"
              }
            ]
          }
        ],
        "metrics": {
          "serviceMonitor": {
            "enabled": true
          }
        },
        "nodeAgent": {
          "extraArgs": [
            "--node-agent-configmap=node-agent-config"
          ]
        },
        "snapshotsEnabled": false
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "velero-system",
    "nsLabels": {
      "pod-security.kubernetes.io/enforce": "privileged"
    },
    "repoURL": "https://vmware-tanzu.github.io/helm-charts",
    "targetRevision": "12.0.0",
    "wave": "0"
  },
  "volume-replicator": {
    "automation": {
      "cleanupOnDeletion": false,
      "enabled": true,
      "prune": true,
      "selfHeal": true,
      "syncOptions": {}
    },
    "enabled": true,
    "helm": {
      "chart": "volume-replicator",
      "releaseName": "volume-replicator",
      "values": {
        "exclusionRegex": "^prime-.*$"
      }
    },
    "modes": [
      "Hyperconverged",
      "DecoupledWorkload"
    ],
    "namespace": "volume-replicator",
    "repoURL": "ghcr.io/super-phenix/helm-charts",
    "targetRevision": "0.5.1"
  }
}
</pre>
</td>
			<td>Per-application settings. Each key defines an Argo CD Application that is generated conditionally based on `enabled` and `modes` (see the README).</td>
		</tr>
		<tr>
			<td>apps.cdi</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "cdi",
    "releaseName": "cdi"
  },
  "modes": [
    "Hyperconverged",
    "DecoupledWorkload"
  ],
  "namespace": "cdi-system",
  "repoURL": "oci://ghcr.io/super-phenix/charts/cdi",
  "targetRevision": "0.1.0",
  "wave": "5"
}
</pre>
</td>
			<td>Containerized Data Importer (CDI) for KubeVirt. Provisions VM disks from images, PVCs or uploads.</td>
		</tr>
		<tr>
			<td>apps.cert-manager</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "cert-manager",
    "releaseName": "cert-manager",
    "values": {
      "crds": {
        "enabled": true
      },
      "enableCertificateOwnerRef": true,
      "extraObjects": [
        "apiVersion: cert-manager.io/v1\nkind: ClusterIssuer\nmetadata:\n  name: letsencrypt\nspec:\n  acme:\n    # The ACME server URL\n    server: https://acme-v02.api.letsencrypt.org/directory\n\n    # Name of a secret used to store the ACME account private key\n    privateKeySecretRef:\n      name: letsencrypt\n\n    # Enable the HTTP-01 challenge provider\n    solvers:\n    - http01:\n        ingress:\n          class: traefik\n"
      ],
      "prometheus": {
        "servicemonitor": {
          "enabled": true
        }
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledStorage",
    "DecoupledWorkload"
  ],
  "namespace": "cert-manager-system",
  "repoURL": "https://charts.jetstack.io",
  "targetRevision": "1.19.1",
  "wave": "-10"
}
</pre>
</td>
			<td>cert-manager: issues and renews TLS certificates, including via Let's Encrypt ACME.</td>
		</tr>
		<tr>
			<td>apps.cilium</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "cilium",
    "releaseName": "cilium",
    "values": {
      "cgroup": {
        "autoMount": {
          "enabled": false
        },
        "hostRoot": "/sys/fs/cgroup"
      },
      "envoy": {
        "enabled": false
      },
      "hubble": {
        "enabled": false
      },
      "ipam": {
        "mode": "kubernetes"
      },
      "ipv4": {
        "enabled": true
      },
      "ipv6": {
        "enabled": true
      },
      "k8s": {
        "requireIPv4PodCIDR": true,
        "requireIPv6PodCIDR": true
      },
      "k8sServiceHost": "localhost",
      "k8sServicePort": 7445,
      "kubeProxyReplacement": true,
      "securityContext": {
        "capabilities": {
          "ciliumAgent": [
            "CHOWN",
            "KILL",
            "NET_ADMIN",
            "NET_RAW",
            "IPC_LOCK",
            "SYS_ADMIN",
            "SYS_RESOURCE",
            "DAC_OVERRIDE",
            "FOWNER",
            "SETGID",
            "SETUID"
          ],
          "cleanCiliumState": [
            "NET_ADMIN",
            "SYS_ADMIN",
            "SYS_RESOURCE"
          ]
        }
      }
    }
  },
  "modes": [
    "DecoupledStorage"
  ],
  "namespace": "kube-system",
  "repoURL": "https://helm.cilium.io/",
  "targetRevision": "1.17.2",
  "wave": "-100"
}
</pre>
</td>
			<td>Cilium CNI, used on storage clusters where the Kube-OVN overlay is not needed. TODO: Consider phasing Cilium out and using Kube-OVN everywhere.</td>
		</tr>
		<tr>
			<td>apps.cluster-api-operator</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "cluster-api-operator",
    "releaseName": "cluster-api-operator",
    "values": {
      "bootstrap": {
        "kubeadm": {
          "version": "v1.12.10"
        }
      },
      "controlPlane": {
        "kamaji": {
          "version": "v0.19.0"
        }
      },
      "core": {
        "cluster-api": {
          "version": "v1.12.10"
        }
      },
      "infrastructure": {
        "kubevirt": {
          "version": "v0.11.2"
        }
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledWorkload"
  ],
  "namespace": "capi-operator-system",
  "repoURL": "https://kubernetes-sigs.github.io/cluster-api-operator",
  "targetRevision": "0.28.0",
  "wave": "15"
}
</pre>
</td>
			<td>Cluster API operator: installs the providers used by the KaaS stack (core, bootstrap, infra, control-plane).</td>
		</tr>
		<tr>
			<td>apps.coredns</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": [
      "SkipDryRunOnMissingResource=true"
    ]
  },
  "enabled": true,
  "helm": {
    "chart": "coredns",
    "releaseName": "coredns",
    "values": {
      "prometheus": {
        "monitor": {
          "enabled": true
        },
        "service": {
          "enabled": true
        }
      },
      "replicaCount": 2,
      "service": {
        "clusterIP": "fd00:100:ffff::a",
        "clusterIPs": [
          "fd00:100:ffff::a",
          "10.16.0.10"
        ],
        "ipFamilyPolicy": "RequireDualStack"
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledStorage",
    "DecoupledWorkload"
  ],
  "namespace": "coredns-system",
  "repoURL": "https://coredns.github.io/helm",
  "targetRevision": "1.37.3",
  "wave": "-100"
}
</pre>
</td>
			<td>CoreDNS used for DNS resolution within the Kubernetes clusters</td>
		</tr>
		<tr>
			<td>apps.csi-addons</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "csi-addons",
    "releaseName": "csi-addons"
  },
  "modes": [
    "Hyperconverged",
    "DecoupledWorkload"
  ],
  "namespace": "csi-addons-system",
  "path": ".",
  "repoURL": "oci://ghcr.io/super-phenix/charts/csi-addons",
  "targetRevision": "0.1.0",
  "wave": "0"
}
</pre>
</td>
			<td>csi-addons: adds features such as volume replication and reclaim-space to the CSI drivers.</td>
		</tr>
		<tr>
			<td>apps.csi-snapshotter</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "external-snapshotter",
    "releaseName": "external-snapshotter"
  },
  "modes": [
    "Hyperconverged",
    "DecoupledWorkload"
  ],
  "namespace": "csi-snapshotter-system",
  "path": ".",
  "repoURL": "oci://ghcr.io/super-phenix/charts/external-snapshotter",
  "targetRevision": "0.1.0",
  "wave": "0"
}
</pre>
</td>
			<td>external-snapshotter: adds VolumeSnapshot support to the CSI drivers.</td>
		</tr>
		<tr>
			<td>apps.etcd-operator</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "etcd-operator",
    "releaseName": "etcd-operator",
    "values": {
      "etcdOperator": {
        "vpa": {
          "enabled": false
        }
      },
      "kubeRbacProxy": {
        "vpa": {
          "enabled": false
        }
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledWorkload"
  ],
  "namespace": "etcd-operator-system",
  "repoURL": "ghcr.io/cozystack/charts",
  "targetRevision": "0.5.3",
  "wave": "5"
}
</pre>
</td>
			<td>etcd-operator: manages the etcd datastores backing Kamaji control planes.</td>
		</tr>
		<tr>
			<td>apps.gateway-api</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "directory": {
    "recurse": true
  },
  "enabled": true,
  "modes": [
    "Hyperconverged",
    "DecoupledWorkload"
  ],
  "namespace": "kube-system",
  "path": "config/crd/standard",
  "repoURL": "https://github.com/kubernetes-sigs/gateway-api.git",
  "targetRevision": "v1.6.1",
  "wave": "0"
}
</pre>
</td>
			<td>Gateway API standard CRDs (installed cluster-wide).</td>
		</tr>
		<tr>
			<td>apps.idrac-exporter</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "enabled": true,
    "prune": true,
    "selfHeal": true
  },
  "enabled": false,
  "helm": {
    "chart": "idrac-exporter",
    "releaseName": "idrac-exporter",
    "values": {}
  },
  "modes": [
    "Management"
  ],
  "repoURL": "https://mrlhansen.github.io/idrac_exporter",
  "targetRevision": "2.6.1"
}
</pre>
</td>
			<td>iDRAC Exporter: Prometheus exporter for Dell iDRAC BMCs. Disabled by default.</td>
		</tr>
		<tr>
			<td>apps.ingress-nginx</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "ingress-nginx",
    "releaseName": "ingress-nginx",
    "values": {
      "controller": {
        "config": {
          "enable-real-ip": "true",
          "hsts": "false",
          "log-format-upstream": "{\"msec\": \"$msec\", \"connection\": \"$connection\", \"connection_requests\": \"$connection_requests\", \"pid\": \"$pid\", \"request_id\": \"$request_id\", \"request_length\": \"$request_length\", \"remote_addr\": \"$remote_addr\", \"remote_user\": \"$remote_user\", \"remote_port\": \"$remote_port\", \"time_local\": \"$time_local\", \"time_iso8601\": \"$time_iso8601\", \"request\": \"$request\", \"request_uri\": \"$request_uri\", \"args\": \"$args\", \"status\": \"$status\", \"body_bytes_sent\": \"$body_bytes_sent\", \"bytes_sent\": \"$bytes_sent\", \"http_referer\": \"$http_referer\", \"http_user_agent\": \"$http_user_agent\", \"http_x_forwarded_for\": \"$http_x_forwarded_for\", \"http_host\": \"$http_host\", \"server_name\": \"$server_name\", \"request_time\": \"$request_time\", \"upstream\": \"$upstream_addr\", \"upstream_connect_time\": \"$upstream_connect_time\", \"upstream_header_time\": \"$upstream_header_time\", \"upstream_response_time\": \"$upstream_response_time\", \"upstream_response_length\": \"$upstream_response_length\", \"upstream_cache_status\": \"$upstream_cache_status\", \"ssl_protocol\": \"$ssl_protocol\", \"ssl_cipher\": \"$ssl_cipher\", \"scheme\": \"$scheme\", \"request_method\": \"$request_method\", \"server_protocol\": \"$server_protocol\", \"pipe\": \"$pipe\", \"gzip_ratio\": \"$gzip_ratio\", \"http_cf_ray\": \"$http_cf_ray\"}",
          "use-forwarded-headers": "true"
        },
        "containerPort": {
          "http": 80,
          "https": 443
        },
        "hostNetwork": true,
        "ingressClass": "spx-nginx",
        "ingressClassResource": {
          "name": "spx-nginx"
        },
        "kind": "DaemonSet",
        "metrics": {
          "enabled": true,
          "serviceMonitor": {
            "additionalLabels": {
              "release": "prometheus"
            },
            "enabled": true
          }
        },
        "service": {
          "ports": {
            "http": 80,
            "https": 443
          },
          "targetPorts": {
            "http": 8080,
            "https": 8443
          },
          "type": "ClusterIP"
        },
        "tolerations": [
          {
            "effect": "NoSchedule",
            "key": "node-role.kubernetes.io/master",
            "operator": "Exists"
          }
        ]
      }
    }
  },
  "modes": [
    "DecoupledStorage"
  ],
  "namespace": "ingress-nginx-system",
  "nsLabels": {
    "pod-security.kubernetes.io/enforce": "privileged"
  },
  "repoURL": "https://kubernetes.github.io/ingress-nginx",
  "targetRevision": "4.13.3",
  "wave": "-10"
}
</pre>
</td>
			<td>ingress-nginx ingress controller. Kept for backwards compatibility with legacy Ingress annotations. TODO: phase out ingress-nginx and rely on Traefik only.</td>
		</tr>
		<tr>
			<td>apps.kaas-controller</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "kaas-controller",
    "releaseName": "kaas-controller"
  },
  "modes": [
    "Hyperconverged",
    "DecoupledVirtualization"
  ],
  "namespace": "kaas-system",
  "repoURL": "ghcr.io/super-phenix/charts",
  "targetRevision": "",
  "wave": "15"
}
</pre>
</td>
			<td>KaaS controller: extra CRDs and controllers backing the Kubernetes-as-a-Service stack.</td>
		</tr>
		<tr>
			<td>apps.kaas-datastore</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "kaas-datastore",
    "releaseName": "kaas-datastore",
    "values": {}
  },
  "modes": [
    "Hyperconverged",
    "DecoupledVirtualization"
  ],
  "namespace": "kaas-datastore-system",
  "repoURL": "ghcr.io/super-phenix/charts",
  "targetRevision": "",
  "wave": "5"
}
</pre>
</td>
			<td>KaaS datastore: provisions etcd clusters (via etcd-operator) used as Kamaji datastores.</td>
		</tr>
		<tr>
			<td>apps.kamaji</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "kamaji",
    "releaseName": "kamaji",
    "values": {
      "affinity": {
        "podAntiAffinity": {
          "preferredDuringSchedulingIgnoredDuringExecution": [
            {
              "podAffinityTerm": {
                "labelSelector": {
                  "matchLabels": {
                    "app.kubernetes.io/name": "kamaji"
                  }
                },
                "topologyKey": "kubernetes.io/hostname"
              },
              "weight": 1
            }
          ]
        }
      },
      "defaultDatastoreName": "",
      "extraArgs": [
        "--certificate-expiration-deadline=336h"
      ],
      "image": {
        "tag": "26.7.3-edge"
      },
      "kamaji-etcd": {
        "deploy": false
      },
      "replicaCount": 3,
      "resources": {
        "limits": {
          "cpu": "1000m",
          "memory": "2Gi"
        },
        "requests": {
          "cpu": "100m",
          "memory": "200Mi"
        }
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledVirtualization"
  ],
  "namespace": "kamaji-system",
  "repoURL": "harbor.agc.dpk-agc-cl04.agoracalyce.net/spx-helm",
  "targetRevision": "26.7.22",
  "wave": "10"
}
</pre>
</td>
			<td>Kamaji: hosted Kubernetes control-plane provider used by the KaaS stack.</td>
		</tr>
		<tr>
			<td>apps.kratos</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "enabled": true,
    "prune": true,
    "selfHeal": true
  },
  "enabled": true,
  "helm": {
    "chart": "kratos",
    "releaseName": "kratos",
    "values": {
      "ingress": {
        "public": {
          "annotations": {},
          "className": "",
          "enabled": true,
          "hosts": [
            {
              "host": "{{ (urlParse $.Values.apps.kratos.helm.values.kratos.config.serve.public.base_url).host }}",
              "paths": [
                {
                  "path": "/",
                  "pathType": "ImplementationSpecific"
                }
              ]
            }
          ],
          "tls": [
            {
              "hosts": [
                "{{ (urlParse $.Values.apps.kratos.helm.values.kratos.config.serve.public.base_url).host }}"
              ],
              "secretName": "kratos-public-tls"
            }
          ]
        }
      },
      "kratos": {
        "automigration": {
          "enabled": true
        },
        "config": {
          "cookies": {
            "domain": "{{ (urlParse $.Values.apps.kratos.helm.values.kratos.config.serve.public.base_url).host }}",
            "same_site": "Strict"
          },
          "dsn": "postgres://superphenix:{{ $.Values.apps.postgres.helm.values.auth.password }}@postgres.{{ $.Release.Namespace }}.svc:5432/kratos?sslmode=disable",
          "identity": {
            "default_schema_id": "default",
            "schemas": [
              {
                "id": "default",
                "url": "file:///etc/config/identity.default.schema.json"
              }
            ]
          },
          "secrets": {
            "default": [
              "base64=="
            ]
          },
          "selfservice": {
            "allowed_return_urls": [
              "https://*.{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/"
            ],
            "default_browser_return_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/",
            "flows": {
              "error": {
                "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/error"
              },
              "login": {
                "after": {
                  "hooks": [
                    {
                      "hook": "require_verified_address"
                    }
                  ]
                },
                "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/login"
              },
              "recovery": {
                "after": {
                  "hooks": [
                    {
                      "hook": "revoke_active_sessions"
                    }
                  ]
                },
                "enabled": true,
                "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/recovery"
              },
              "registration": {
                "after": {
                  "code": {
                    "hooks": [
                      {
                        "hook": "show_verification_ui"
                      }
                    ]
                  },
                  "password": {
                    "hooks": [
                      {
                        "hook": "show_verification_ui"
                      }
                    ]
                  },
                  "webauthn": {
                    "hooks": [
                      {
                        "hook": "show_verification_ui"
                      }
                    ]
                  }
                },
                "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/registration"
              },
              "settings": {
                "privileged_session_max_age": "15m",
                "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/settings"
              },
              "verification": {
                "enabled": true,
                "lifespan": "1h",
                "notify_unknown_recipients": false,
                "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/verification",
                "use": "code"
              }
            },
            "methods": {
              "code": {
                "config": {
                  "lifespan": "15m"
                },
                "enabled": true
              }
            }
          },
          "serve": {
            "public": {
              "base_url": "https://auth.console.superphenix.net",
              "cors": {
                "allow_credentials": true,
                "allowed_headers": [
                  "Authorization",
                  "Cookie",
                  "Content-Type",
                  "Max-Age",
                  "X-Session-Token",
                  "X-XSRF-TOKEN",
                  "X-CSRF-TOKEN"
                ],
                "allowed_methods": [
                  "POST",
                  "GET",
                  "PUT",
                  "PATCH",
                  "DELETE"
                ],
                "allowed_origins": [
                  "https://*.{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}",
                  "https://*.{{ (index $.Values.apps \"superphenix-api\").helm.values.domain }}"
                ],
                "enabled": true,
                "exposed_headers": [
                  "Content-Type",
                  "Set-Cookie"
                ]
              },
              "request_log": {
                "disable_for_health": true
              }
            }
          }
        },
        "identitySchemas": {
          "identity.default.schema.json": "{\n  \"$id\": \"https://schemas.ory.sh/presets/kratos/identity.email.schema.json\",\n  \"$schema\": \"http://json-schema.org/draft-07/schema#\",\n  \"title\": \"Person\",\n  \"type\": \"object\",\n  \"properties\": {\n    \"traits\": {\n      \"type\": \"object\",\n\n      \"properties\": {\n        \"email\": {\n          \"type\": \"string\",\n          \"format\": \"email\",\n          \"title\": \"Email\",\n          \"ory.sh/kratos\": {\n            \"credentials\": {\n              \"password\": {\n                \"identifier\": true\n              }\n            },\n            \"recovery\": {\n              \"via\": \"email\"\n            },\n            \"verification\": {\n              \"via\": \"email\"\n            }\n          }\n        },\n        \"name\": {\n          \"type\": \"object\",\n          \"properties\": {\n            \"first\": {\n              \"type\": \"string\",\n              \"title\": \"First name\"\n            },\n            \"last\": {\n              \"type\": \"string\",\n              \"title\": \"Last name\"\n            }\n          }\n        }\n      },\n      \"required\": [\"email\"],\n      \"additionalProperties\": false\n    }\n  }\n}\n"
        }
      }
    }
  },
  "modes": [
    "Management"
  ],
  "repoURL": "https://k8s.ory.sh/helm/charts",
  "targetRevision": "0.43.1"
}
</pre>
</td>
			<td>Ory Kratos: identity, session and self-service flows backing the Superphenix console.</td>
		</tr>
		<tr>
			<td>apps.kratos.helm.values.kratos.automigration</td>
			<td>object</td>
			<td><pre lang="json">
{
  "enabled": true
}
</pre>
</td>
			<td>Run database migrations automatically on Kratos upgrades.</td>
		</tr>
		<tr>
			<td>apps.kratos.helm.values.kratos.config.cookies</td>
			<td>object</td>
			<td><pre lang="json">
{
  "domain": "{{ (urlParse $.Values.apps.kratos.helm.values.kratos.config.serve.public.base_url).host }}",
  "same_site": "Strict"
}
</pre>
</td>
			<td>Session cookie settings.</td>
		</tr>
		<tr>
			<td>apps.kratos.helm.values.kratos.config.dsn</td>
			<td>string</td>
			<td><pre lang="json">
"postgres://superphenix:{{ $.Values.apps.postgres.helm.values.auth.password }}@postgres.{{ $.Release.Namespace }}.svc:5432/kratos?sslmode=disable"
</pre>
</td>
			<td>Kratos DSN stores users, sessions, recovery codes and verification data.</td>
		</tr>
		<tr>
			<td>apps.kratos.helm.values.kratos.config.identity</td>
			<td>object</td>
			<td><pre lang="json">
{
  "default_schema_id": "default",
  "schemas": [
    {
      "id": "default",
      "url": "file:///etc/config/identity.default.schema.json"
    }
  ]
}
</pre>
</td>
			<td>Identity schemas used at registration.</td>
		</tr>
		<tr>
			<td>apps.kratos.helm.values.kratos.config.secrets</td>
			<td>object</td>
			<td><pre lang="json">
{
  "default": [
    "base64=="
  ]
}
</pre>
</td>
			<td>Kratos signing secrets. MUST be overridden per environment with a strong random value.</td>
		</tr>
		<tr>
			<td>apps.kratos.helm.values.kratos.config.selfservice.allowed_return_urls</td>
			<td>list</td>
			<td><pre lang="json">
[
  "https://*.{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/"
]
</pre>
</td>
			<td>Allowed post-flow redirect targets (glob-based).</td>
		</tr>
		<tr>
			<td>apps.kratos.helm.values.kratos.config.selfservice.default_browser_return_url</td>
			<td>string</td>
			<td><pre lang="json">
"https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/"
</pre>
</td>
			<td>Default post-flow redirect target.</td>
		</tr>
		<tr>
			<td>apps.kratos.helm.values.kratos.config.selfservice.flows</td>
			<td>object</td>
			<td><pre lang="json">
{
  "error": {
    "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/error"
  },
  "login": {
    "after": {
      "hooks": [
        {
          "hook": "require_verified_address"
        }
      ]
    },
    "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/login"
  },
  "recovery": {
    "after": {
      "hooks": [
        {
          "hook": "revoke_active_sessions"
        }
      ]
    },
    "enabled": true,
    "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/recovery"
  },
  "registration": {
    "after": {
      "code": {
        "hooks": [
          {
            "hook": "show_verification_ui"
          }
        ]
      },
      "password": {
        "hooks": [
          {
            "hook": "show_verification_ui"
          }
        ]
      },
      "webauthn": {
        "hooks": [
          {
            "hook": "show_verification_ui"
          }
        ]
      }
    },
    "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/registration"
  },
  "settings": {
    "privileged_session_max_age": "15m",
    "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/settings"
  },
  "verification": {
    "enabled": true,
    "lifespan": "1h",
    "notify_unknown_recipients": false,
    "ui_url": "https://{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}/ui/verification",
    "use": "code"
  }
}
</pre>
</td>
			<td>Kratos self-service flow configuration (login, registration, recovery, ...).</td>
		</tr>
		<tr>
			<td>apps.kratos.helm.values.kratos.config.selfservice.methods</td>
			<td>object</td>
			<td><pre lang="json">
{
  "code": {
    "config": {
      "lifespan": "15m"
    },
    "enabled": true
  }
}
</pre>
</td>
			<td>Enable code-based (one-time code) authentication.</td>
		</tr>
		<tr>
			<td>apps.kratos.helm.values.kratos.config.serve</td>
			<td>object</td>
			<td><pre lang="json">
{
  "public": {
    "base_url": "https://auth.console.superphenix.net",
    "cors": {
      "allow_credentials": true,
      "allowed_headers": [
        "Authorization",
        "Cookie",
        "Content-Type",
        "Max-Age",
        "X-Session-Token",
        "X-XSRF-TOKEN",
        "X-CSRF-TOKEN"
      ],
      "allowed_methods": [
        "POST",
        "GET",
        "PUT",
        "PATCH",
        "DELETE"
      ],
      "allowed_origins": [
        "https://*.{{ (index $.Values.apps \"superphenix-console\").helm.values.domain }}",
        "https://*.{{ (index $.Values.apps \"superphenix-api\").helm.values.domain }}"
      ],
      "enabled": true,
      "exposed_headers": [
        "Content-Type",
        "Set-Cookie"
      ]
    },
    "request_log": {
      "disable_for_health": true
    }
  }
}
</pre>
</td>
			<td>Kratos public/admin listener configuration.</td>
		</tr>
		<tr>
			<td>apps.kratos.helm.values.kratos.identitySchemas</td>
			<td>object</td>
			<td><pre lang="json">
{
  "identity.default.schema.json": "{\n  \"$id\": \"https://schemas.ory.sh/presets/kratos/identity.email.schema.json\",\n  \"$schema\": \"http://json-schema.org/draft-07/schema#\",\n  \"title\": \"Person\",\n  \"type\": \"object\",\n  \"properties\": {\n    \"traits\": {\n      \"type\": \"object\",\n\n      \"properties\": {\n        \"email\": {\n          \"type\": \"string\",\n          \"format\": \"email\",\n          \"title\": \"Email\",\n          \"ory.sh/kratos\": {\n            \"credentials\": {\n              \"password\": {\n                \"identifier\": true\n              }\n            },\n            \"recovery\": {\n              \"via\": \"email\"\n            },\n            \"verification\": {\n              \"via\": \"email\"\n            }\n          }\n        },\n        \"name\": {\n          \"type\": \"object\",\n          \"properties\": {\n            \"first\": {\n              \"type\": \"string\",\n              \"title\": \"First name\"\n            },\n            \"last\": {\n              \"type\": \"string\",\n              \"title\": \"Last name\"\n            }\n          }\n        }\n      },\n      \"required\": [\"email\"],\n      \"additionalProperties\": false\n    }\n  }\n}\n"
}
</pre>
</td>
			<td>Contents of the identity schema files referenced by `config.identity.schemas`.</td>
		</tr>
		<tr>
			<td>apps.kubeovn</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "kube-ovn-v2",
    "releaseName": "kube-ovn",
    "values": {
      "apiNad": {
        "enabled": true
      },
      "extraObjects": [
        {
          "apiVersion": "kubeovn.io/v1",
          "kind": "Subnet",
          "metadata": {
            "name": "system-blackhole"
          },
          "spec": {
            "acls": [
              {
                "action": "drop",
                "direction": "from-lport",
                "match": "ip"
              },
              {
                "action": "drop",
                "direction": "to-lport",
                "match": "ip"
              }
            ],
            "cidrBlock": "fd00:666::/64",
            "disableGatewayCheck": true,
            "protocol": "IPv6",
            "provider": "system-blackhole.kube-system.ovn",
            "vpc": "system-blackhole-vpc"
          }
        },
        {
          "apiVersion": "kubeovn.io/v1",
          "kind": "Vpc",
          "metadata": {
            "name": "system-blackhole-vpc"
          },
          "spec": {}
        },
        {
          "apiVersion": "k8s.cni.cncf.io/v1",
          "kind": "NetworkAttachmentDefinition",
          "metadata": {
            "name": "system-blackhole",
            "namespace": "kube-system"
          },
          "spec": {
            "config": "{ \"cniVersion\": \"0.3.0\", \"type\": \"kube-ovn\", \"server_socket\": \"/run/openvswitch/kube-ovn-daemon.sock\", \"provider\": \"system-blackhole.kube-system.ovn\" }"
          }
        },
        {
          "apiVersion": "kubeovn.io/v1",
          "kind": "Subnet",
          "metadata": {
            "name": "system-isolated-egress"
          },
          "spec": {
            "acls": [
              {
                "action": "allow-related",
                "direction": "from-lport",
                "match": "ip4.dst == 10.16.0.10 || ip6.dst == fd00:100:ffff::a",
                "priority": 1001
              },
              {
                "action": "allow-related",
                "direction": "from-lport",
                "match": "ip4.src == 10.32.0.1 || ip6.src == fd00:110::1",
                "priority": 1001
              },
              {
                "action": "drop",
                "direction": "from-lport",
                "match": "ip4.dst == 10.0.0.0/8 \u0026\u0026 ip4.dst != 10.32.0.1 \u0026\u0026 ip4.src == 10.0.0.0/8",
                "priority": 1000
              },
              {
                "action": "drop",
                "direction": "from-lport",
                "match": "ip4.dst == 172.16.0.0/12 \u0026\u0026 ip",
                "priority": 1000
              },
              {
                "action": "drop",
                "direction": "from-lport",
                "match": "ip4.dst == 192.168.0.0/16 \u0026\u0026 ip",
                "priority": 1000
              },
              {
                "action": "drop",
                "direction": "from-lport",
                "match": "ip4.dst == 100.64.0.0/10 \u0026\u0026 ip",
                "priority": 1000
              },
              {
                "action": "drop",
                "direction": "from-lport",
                "match": "ip4.dst == 169.254.0.0/16 \u0026\u0026 ip",
                "priority": 1000
              },
              {
                "action": "drop",
                "direction": "from-lport",
                "match": "ip6.dst == fc00::/7 \u0026\u0026 ip6.dst != fd00:110::1 \u0026\u0026 ip6.src == fd00:110::/64",
                "priority": 1000
              }
            ],
            "allowEWTraffic": false,
            "cidrBlock": "10.32.0.0/16,fd00:110::/64",
            "gatewayType": "distributed",
            "mtu": 1380,
            "natOutgoing": true,
            "protocol": "Dual",
            "provider": "system-isolated-egress.kube-system.ovn",
            "vpc": "ovn-cluster"
          }
        },
        {
          "apiVersion": "k8s.cni.cncf.io/v1",
          "kind": "NetworkAttachmentDefinition",
          "metadata": {
            "name": "system-isolated-egress",
            "namespace": "kube-system"
          },
          "spec": {
            "config": "{ \"cniVersion\": \"0.3.0\", \"type\": \"kube-ovn\", \"server_socket\": \"/run/openvswitch/kube-ovn-daemon.sock\", \"provider\": \"system-isolated-egress.kube-system.ovn\" }"
          }
        }
      ],
      "features": {
        "enableNetworkPolicies": true
      },
      "masterNodes": "invalid,invalid,invalid",
      "masterNodesLabels": {
        "kube-ovn/role": null,
        "node-role.kubernetes.io/control-plane": ""
      },
      "natGw": {
        "namePrefix": "nat-gateway"
      },
      "networking": {
        "join": {
          "cidr": {
            "v4": "100.64.0.0/12",
            "v6": "fd00:100:64::/112"
          }
        },
        "pods": {
          "cidr": {
            "v4": "10.0.0.0/12",
            "v6": "fd00:100:0000:0::/96"
          },
          "gateways": {
            "v4": "10.0.0.1",
            "v6": "fd00:100:0000:0::1"
          }
        },
        "services": {
          "cidr": {
            "v4": "10.16.0.0/12",
            "v6": "fd00:100:ffff:0::/112"
          }
        },
        "stack": "Dual"
      },
      "ovsOvn": {
        "disableModulesManagement": true,
        "ovnDirectory": "/var/lib/ovn",
        "ovsDirectory": "/var/lib/openvswitch"
      },
      "validatingWebhook": {
        "enabled": true
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledVirtualization"
  ],
  "namespace": "kube-system",
  "repoURL": "oci://ghcr.io/kubeovn/charts/kube-ovn-v2",
  "targetRevision": "v1.16.0",
  "wave": "-100"
}
</pre>
</td>
			<td>Kube-OVN is used as the CNI for the virtualization layer</td>
		</tr>
		<tr>
			<td>apps.kubevirt</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "directory": {
    "recurse": true
  },
  "enabled": true,
  "modes": [
    "Hyperconverged",
    "DecoupledVirtualization"
  ],
  "namespace": "kubevirt-system",
  "path": "v1.7.3/",
  "repoURL": "https://github.com/super-phenix/kubevirt-manifests.git",
  "targetRevision": "HEAD",
  "wave": "5"
}
</pre>
</td>
			<td>KubeVirt: virtualization runtime that lets Kubernetes schedule VMs alongside containers.</td>
		</tr>
		<tr>
			<td>apps.kyverno</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "kyverno",
    "releaseName": "kyverno",
    "values": {
      "reportsController": {
        "enabled": false
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledVirtualization"
  ],
  "namespace": "kyverno-system",
  "repoURL": "https://kyverno.github.io/kyverno/",
  "targetRevision": "3.6.0",
  "wave": "0"
}
</pre>
</td>
			<td>Kyverno policy engine. Currently only used to patch VolumeSnapshotClass/VolumeReplicationClass at runtime; this dependency should be removed as soon as possible.</td>
		</tr>
		<tr>
			<td>apps.loki</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "loki",
    "releaseName": "loki",
    "values": {
      "backend": {
        "replicas": 0
      },
      "bloomCompactor": {
        "replicas": 0
      },
      "bloomGateway": {
        "replicas": 0
      },
      "chunksCache": {
        "enabled": false
      },
      "compactor": {
        "replicas": 0
      },
      "deploymentMode": "SingleBinary",
      "distributor": {
        "replicas": 0
      },
      "gateway": {
        "enabled": false
      },
      "global": {
        "clusterDomain": "",
        "dnsNamespace": "kube-system",
        "dnsService": "coredns"
      },
      "indexGateway": {
        "replicas": 0
      },
      "ingester": {
        "replicas": 0
      },
      "loki": {
        "auth_enabled": true,
        "commonConfig": {
          "replication_factor": 1
        },
        "compactor": {
          "compaction_interval": "10m",
          "delete_request_store": "filesystem",
          "retention_delete_delay": "2h",
          "retention_delete_worker_count": 150,
          "retention_enabled": true
        },
        "limits_config": {
          "retention_period": "14d"
        },
        "schemaConfig": {
          "configs": [
            {
              "from": "2024-12-01",
              "index": {
                "period": "24h",
                "prefix": "loki_index_"
              },
              "object_store": "filesystem",
              "schema": "v13",
              "store": "tsdb"
            }
          ]
        },
        "storage": {
          "type": "filesystem"
        }
      },
      "lokiCanary": {
        "enabled": false
      },
      "minio": {
        "enabled": false
      },
      "monitoring": {
        "serviceMonitor": {
          "enabled": true
        }
      },
      "querier": {
        "replicas": 0
      },
      "queryFrontend": {
        "replicas": 0
      },
      "queryScheduler": {
        "replicas": 0
      },
      "read": {
        "replicas": 0
      },
      "resultsCache": {
        "enabled": false
      },
      "singleBinary": {
        "persistence": {
          "enableStatefulSetAutoDeletePVC": false,
          "size": "50Gi"
        },
        "replicas": 1
      },
      "test": {
        "enabled": false
      },
      "write": {
        "replicas": 0
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledStorage",
    "DecoupledVirtualization"
  ],
  "namespace": "loki-system",
  "repoURL": "https://grafana.github.io/helm-charts",
  "targetRevision": "6.43.0",
  "wave": "-5"
}
</pre>
</td>
			<td>Loki: log aggregation backend with a Prometheus-like query language (LogQL).</td>
		</tr>
		<tr>
			<td>apps.multus</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "multus",
    "releaseName": "multus",
    "values": {
      "tolerations": [
        {
          "effect": "NoSchedule",
          "operator": "Exists"
        },
        {
          "effect": "NoExecute",
          "operator": "Exists"
        }
      ]
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledVirtualization"
  ],
  "namespace": "kube-system",
  "repoURL": "oci://ghcr.io/super-phenix/charts/multus",
  "targetRevision": "0.1.0",
  "wave": "-100"
}
</pre>
</td>
			<td>Multus CNI meta-plugin, enables multi-homed pods and VMs.</td>
		</tr>
		<tr>
			<td>apps.permify</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "enabled": true,
    "prune": true,
    "selfHeal": true
  },
  "enabled": true,
  "helm": {
    "chart": "permify",
    "releaseName": "permify",
    "values": {
      "app": {
        "database": {
          "engine": "postgres",
          "garbage_collection": {
            "enabled": false
          },
          "uri": "postgres://superphenix:{{ (index $.Values.apps \"postgres\").helm.values.auth.password }}@postgres.{{ $.Release.Namespace }}:5432/permify"
        },
        "distributed": {
          "address": "permify.{{ $.Release.Namespace }}.svc:5000",
          "enabled": false,
          "port": 5000
        }
      }
    }
  },
  "modes": [
    "Management"
  ],
  "repoURL": "https://permify.github.io/helm-charts",
  "targetRevision": "0.3.*"
}
</pre>
</td>
			<td>Permify: authorization service consulted by the Superphenix API to enforce permissions.</td>
		</tr>
		<tr>
			<td>apps.permify.helm.values.app.database</td>
			<td>object</td>
			<td><pre lang="json">
{
  "engine": "postgres",
  "garbage_collection": {
    "enabled": false
  },
  "uri": "postgres://superphenix:{{ (index $.Values.apps \"postgres\").helm.values.auth.password }}@postgres.{{ $.Release.Namespace }}:5432/permify"
}
</pre>
</td>
			<td>Database connection used to store Permify tuples and schemas.</td>
		</tr>
		<tr>
			<td>apps.permify.helm.values.app.distributed</td>
			<td>object</td>
			<td><pre lang="json">
{
  "address": "permify.{{ $.Release.Namespace }}.svc:5000",
  "enabled": false,
  "port": 5000
}
</pre>
</td>
			<td>Distributed mode is disabled (single-instance deployment).</td>
		</tr>
		<tr>
			<td>apps.policies</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "spx-policies",
    "releaseName": "spx-policies"
  },
  "modes": [
    "Hyperconverged",
    "DecoupledVirtualization"
  ],
  "namespace": "kube-system",
  "repoURL": "oci://ghcr.io/super-phenix/charts/spx-policies",
  "targetRevision": "0.1.0"
}
</pre>
</td>
			<td>spx-policies: Kyverno policies that enforce Superphenix RFCs and protect against known bugs and attacks.</td>
		</tr>
		<tr>
			<td>apps.postgres</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "enabled": true,
    "prune": true,
    "selfHeal": true
  },
  "enabled": true,
  "helm": {
    "chart": "postgres",
    "releaseName": "postgres",
    "values": {
      "auth": {
        "database": "superphenix",
        "password": "changeme!",
        "username": "superphenix"
      },
      "initdb": {
        "scripts": {
          "setup.sql": "CREATE DATABASE kratos;\nCREATE DATABASE permify;\nGRANT ALL PRIVILEGES ON DATABASE kratos TO superphenix;\nGRANT ALL PRIVILEGES ON DATABASE permify TO superphenix;\n"
        }
      },
      "persistence": {
        "size": "8Gi"
      }
    }
  },
  "modes": [
    "Management"
  ],
  "repoURL": "oci://registry-1.docker.io/cloudpirates/postgres",
  "targetRevision": "0.19.12"
}
</pre>
</td>
			<td>PostgreSQL: shared database backing the Superphenix API, Kratos and Permify.</td>
		</tr>
		<tr>
			<td>apps.postgres.helm.values.auth</td>
			<td>object</td>
			<td><pre lang="json">
{
  "database": "superphenix",
  "password": "changeme!",
  "username": "superphenix"
}
</pre>
</td>
			<td>Primary user/database credentials. SECURITY: `password` MUST be overridden per environment.</td>
		</tr>
		<tr>
			<td>apps.postgres.helm.values.initdb</td>
			<td>object</td>
			<td><pre lang="json">
{
  "scripts": {
    "setup.sql": "CREATE DATABASE kratos;\nCREATE DATABASE permify;\nGRANT ALL PRIVILEGES ON DATABASE kratos TO superphenix;\nGRANT ALL PRIVILEGES ON DATABASE permify TO superphenix;\n"
  }
}
</pre>
</td>
			<td>Additional databases created at initialization for Kratos and Permify.</td>
		</tr>
		<tr>
			<td>apps.postgres.helm.values.persistence</td>
			<td>object</td>
			<td><pre lang="json">
{
  "size": "8Gi"
}
</pre>
</td>
			<td>Persistent volume size. Sufficient for most deployments.</td>
		</tr>
		<tr>
			<td>apps.prometheus-stack</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": [
      "ServerSideApply=true"
    ]
  },
  "enabled": true,
  "helm": {
    "chart": "kube-prometheus-stack",
    "releaseName": "prometheus",
    "values": {
      "alertmanager": {
        "enabled": false
      },
      "defaultRules": {
        "create": false
      },
      "grafana": {
        "additionalDataSources": [
          {
            "editable": false,
            "jsonData": {
              "httpHeaderName1": "X-Scope-OrgID"
            },
            "name": "Loki",
            "secureJsonData": {
              "httpHeaderValue1": "1"
            },
            "type": "loki",
            "url": "http://loki.loki-system:3100"
          }
        ],
        "adminPassword": "",
        "ingress": {
          "annotations": {
            "cert-manager.io/cluster-issuer": "letsencrypt"
          },
          "enabled": true,
          "hosts": [
            "invalid"
          ],
          "tls": [
            {
              "hosts": [
                "invalid"
              ],
              "secretName": "grafana-tls"
            }
          ]
        }
      },
      "kubeControllerManager": {
        "service": {
          "selector": {
            "k8s-app": "kube-controller-manager"
          }
        }
      },
      "kubeEtcd": {
        "service": {
          "selector": {
            "k8s-app": "kube-controller-manager"
          }
        ]
      }
    }
  },
  "modes": [
    "DecoupledStorage"
  ],
  "namespace": "ingress-nginx-system",
  "nsLabels": {
    "pod-security.kubernetes.io/enforce": "privileged"
  },
  "repoURL": "https://kubernetes.github.io/ingress-nginx",
  "targetRevision": "4.13.3",
  "wave": "-10"
}
</pre>
</td>
			<td>ingress-nginx ingress controller. Kept for backwards compatibility with legacy Ingress annotations. TODO: phase out ingress-nginx and rely on Traefik only.</td>
		</tr>
		<tr>
			<td>apps.kaas-controller</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "kaas-controller",
    "releaseName": "kaas-controller"
  },
  "modes": [
    "Hyperconverged",
    "DecoupledWorkload"
  ],
  "namespace": "kaas-system",
  "repoURL": "ghcr.io/super-phenix/charts",
  "targetRevision": "",
  "wave": "15"
}
</pre>
</td>
			<td>KaaS controller: extra CRDs and controllers backing the Kubernetes-as-a-Service stack.</td>
		</tr>
		<tr>
			<td>apps.kaas-datastore</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "kaas-datastore",
    "releaseName": "kaas-datastore",
    "values": {}
  },
  "modes": [
    "Hyperconverged",
    "DecoupledWorkload"
  ],
  "namespace": "kaas-datastore-system",
  "repoURL": "ghcr.io/super-phenix/charts",
  "targetRevision": "",
  "wave": "5"
}
</pre>
</td>
			<td>KaaS datastore: provisions etcd clusters (via etcd-operator) used as Kamaji datastores.</td>
		</tr>
		<tr>
			<td>apps.kamaji</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "kamaji",
    "releaseName": "kamaji",
    "values": {
      "affinity": {
        "podAntiAffinity": {
          "preferredDuringSchedulingIgnoredDuringExecution": [
            {
              "podAffinityTerm": {
                "labelSelector": {
                  "matchLabels": {
                    "app.kubernetes.io/name": "kamaji"
                  }
                },
                "topologyKey": "kubernetes.io/hostname"
              },
              "weight": 1
            }
          ]
        }
      },
      "defaultDatastoreName": "",
      "extraArgs": [
        "--certificate-expiration-deadline=336h"
      ],
      "image": {
        "tag": "26.7.3-edge"
      },
      "kamaji-etcd": {
        "deploy": false
      },
      "replicaCount": 3,
      "resources": {
        "limits": {
          "cpu": "1000m",
          "memory": "2Gi"
        },
        "serviceMonitor": {
          "metricRelabelings": [
            {
              "action": "labeldrop",
              "regex": "pod"
            }
          ],
          "relabelings": [
            {
              "action": "replace",
              "regex": "^(.*)$",
              "replacement": "$1",
              "separator": ";",
              "sourceLabels": [
                "__meta_kubernetes_pod_node_name"
              ],
              "targetLabel": "nodename"
            }
          ]
        }
      },
      "kubeScheduler": {
        "service": {
          "selector": {
            "k8s-app": "kube-scheduler"
          }
        }
      },
      "prometheus": {
        "prometheusSpec": {
          "podMonitorSelectorNilUsesHelmValues": false,
          "retention": "30d",
          "retentionSize": "45GB",
          "serviceMonitorSelectorNilUsesHelmValues": false,
          "storageSpec": {
            "volumeClaimTemplate": {
              "spec": {
                "resources": {
                  "requests": {
                    "storage": "50Gi"
                  }
                }
              }
            }
          }
        }
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledStorage",
    "DecoupledWorkload"
  ],
  "namespace": "prometheus-system",
  "nsLabels": {
    "pod-security.kubernetes.io/enforce": "privileged"
  },
  "repoURL": "https://prometheus-community.github.io/helm-charts",
  "targetRevision": "78.4.0",
  "wave": "-5"
}
</pre>
</td>
			<td>kube-prometheus-stack: metrics collection, storage and Grafana dashboards.</td>
		</tr>
		<tr>
			<td>apps.promtail</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "promtail",
    "releaseName": "promtail",
    "values": {
      "config": {
        "clients": [
          {
            "tenant_id": 1,
            "url": "http://loki.loki-system:3100/loki/api/v1/push"
          }
        ]
      },
      "serviceMonitor": {
        "enabled": true
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledStorage",
    "DecoupledWorkload"
  ],
  "namespace": "promtail-system",
  "nsLabels": {
    "pod-security.kubernetes.io/enforce": "privileged"
  },
  "repoURL": "https://grafana.github.io/helm-charts",
  "targetRevision": "6.17.0",
  "wave": "0"
}
</pre>
</td>
			<td>Promtail: node-local agent that ships container logs to Loki.</td>
		</tr>
		<tr>
			<td>apps.rook-connection</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": [
      "RespectIgnoreDifferences=true"
    ]
  },
  "enabled": true,
  "helm": {
    "chart": "spx-rook-connection",
    "releaseName": "spx-rook-connection"
  },
  "ignoreDifferences": [
    {
      "jsonPointers": [
        "/data/data",
        "/data/mapping"
      ],
      "kind": "ConfigMap"
    }
  ],
  "modes": [
    "DecoupledWorkload"
  ],
  "namespace": "rook-system",
  "nsLabels": {
    "pod-security.kubernetes.io/enforce": "privileged"
  },
  "path": ".",
  "repoURL": "oci://ghcr.io/super-phenix/charts/spx-rook-connection",
  "targetRevision": "0.2.0",
  "wave": "0"
}
</pre>
</td>
			<td>Rook connection to an external (decoupled) Ceph cluster. Deployed on workload clusters that consume storage from a remote storage cluster.</td>
		</tr>
		<tr>
			<td>apps.rook-local-cluster</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "rook-ceph-cluster",
    "releaseName": "rook-local-cluster",
    "values": {
      "cephBlockPools": [
        {
          "name": "mgr",
          "spec": {
            "deviceClass": "nvme",
            "enableCrushUpdates": true,
            "failureDomain": "host",
            "mirroring": {
              "enabled": false
            },
            "name": ".mgr",
            "parameters": {
              "compression_mode": "none"
            },
            "replicated": {
              "requireSafeReplicaSize": true,
              "size": 3
            }
          },
          "storageClass": {
            "enabled": false
          }
        },
        {
          "name": "spx-scratchpad",
          "spec": {
            "deviceClass": "nvme",
            "enableCrushUpdates": true,
            "enableRBDStats": true,
            "failureDomain": "host",
            "replicated": {
              "size": 2
            }
          },
          "storageClass": {
            "allowVolumeExpansion": true,
            "enabled": true,
            "isDefault": true,
            "mountOptions": [
              "discard"
            ],
            "name": "spx-scratchpad",
            "parameters": {
              "csi.storage.k8s.io/controller-expand-secret-name": "rook-csi-rbd-provisioner",
              "csi.storage.k8s.io/controller-expand-secret-namespace": "spx-storage",
              "csi.storage.k8s.io/fstype": "ext4",
              "csi.storage.k8s.io/node-stage-secret-name": "rook-csi-rbd-node",
              "csi.storage.k8s.io/node-stage-secret-namespace": "spx-storage",
              "csi.storage.k8s.io/provisioner-secret-name": "rook-csi-rbd-provisioner",
              "csi.storage.k8s.io/provisioner-secret-namespace": "spx-storage",
              "imageFeatures": "layering,fast-diff,object-map,deep-flatten,exclusive-lock",
              "imageFormat": "2"
            },
            "reclaimPolicy": "Delete",
            "volumeBindingMode": "Immediate"
          }
        }
      ],
      "cephClusterSpec": {
        "cephConfig": {
          "global": {
            "rbd_mirroring_delete_delay": "604800",
            "rbd_mirroring_max_mirroring_snapshots": "30",
            "rbd_move_to_trash_on_remove": "true",
            "rbd_move_to_trash_on_remove_expire_seconds": "604800"
          }
        },
        "crashCollector": {
          "daysToRetain": 365,
          "disable": false
        },
        "dashboard": {
          "enabled": true,
          "prometheusEndpoint": "http://prometheus-kube-prometheus-prometheus.prometheus-system:9090",
          "prometheusEndpointSSLVerify": false,
          "ssl": false
        },
        "mgr": {
          "modules": [
            {
              "enabled": true,
              "name": "rook"
            }
          ]
        },
        "network": {
          "addressRanges": {
            "cluster": [
              "invalid"
            ],
            "public": [
              "invalid"
            ]
          },
          "ipFamily": "IPv6",
          "provider": "host"
        },
        "storage": {
          "deviceFilter": "",
          "useAllDevices": false
        }
      },
      "cephFileSystems": [],
      "cephObjectStores": [],
      "clusterName": "invalid",
      "ingress": {
        "dashboard": {
          "annotations": {
            "cert-manager.io/cluster-issuer": "letsencrypt"
          },
          "host": {
            "name": "invalid",
            "path": "/"
          },
          "tls": [
            {
              "hosts": [
                "invalid"
              ],
              "secretName": "ceph-dashboard-tls"
            }
          ]
        }
      },
      "monitoring": {
        "enabled": true
      },
      "operatorNamespace": "rook-system",
      "toolbox": {
        "enabled": true
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledStorage"
  ],
  "namespace": "spx-storage",
  "nsLabels": {
    "pod-security.kubernetes.io/enforce": "privileged"
  },
  "repoURL": "https://charts.rook.io/release",
  "targetRevision": "1.16.3",
  "wave": "0"
}
</pre>
</td>
			<td>Local Rook/Ceph cluster deployed on storage-capable clusters (hyperconverged or decoupled storage).</td>
		</tr>
		<tr>
			<td>apps.rook-operator</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "rook-ceph",
    "releaseName": "rook-operator",
    "values": {
      "csi": {
        "csiAddons": {
          "enabled": true
        },
        "csiAddonsRBDProvisionerPort": 9071,
        "enableCephfsDriver": false,
        "enableOMAPGenerator": true,
        "serviceMonitor": {
          "enabled": true
        }
      },
      "discoveryDaemonInterval": "5m",
      "enableDiscoveryDaemon": true,
      "enforceHostNetwork": true,
      "monitoring": {
        "enabled": true
      },
      "obcAllowAdditionalConfigFields": "maxObjects,maxSize,bucketMaxObjects,bucketMaxSize,bucketPolicy,bucketLifecycle",
      "useOperatorHostNetwork": true
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledStorage"
  ],
  "namespace": "spx-storage",
  "nsLabels": {
    "pod-security.kubernetes.io/enforce": "privileged"
  },
  "repoURL": "https://charts.rook.io/release",
  "targetRevision": "1.16.5",
  "wave": "-5"
}
</pre>
</td>
			<td>Rook operator, required on any cluster that interacts with Ceph/Rook. Virtualization clusters need it to provision an external connection to a centralized storage cluster; centralized storage clusters need it to provision a local Ceph cluster that acts as the remote backend for "client" clusters.</td>
		</tr>
		<tr>
			<td>apps.snapscheduler</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "snapscheduler",
    "releaseName": "snapscheduler",
    "values": {
      "enableOwnerReferences": true,
      "replicaCount": 2
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledStorage",
    "DecoupledWorkload"
  ],
  "namespace": "snapscheduler-system",
  "repoURL": "https://backube.github.io/helm-charts/",
  "targetRevision": "*",
  "wave": "0"
}
</pre>
</td>
			<td>snapscheduler: periodic VolumeSnapshot scheduler for PVCs.</td>
		</tr>
		<tr>
			<td>apps.superphenix-api</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "superphenix-api",
    "releaseName": "superphenix-api",
    "values": {
      "config": {
        "database": {
          "database": "superphenix",
          "host": "postgres.{{ $.Release.Namespace }}.svc",
          "password": "{{ $.Values.apps.postgres.helm.values.auth.password | quote }}",
          "port": 5432,
          "username": "superphenix"
        },
        "permify": {
          "url": "permify.{{ $.Release.Namespace }}.svc:3478"
        }
      },
      "domain": "api.superphenix.net"
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledWorkload"
  ],
  "repoURL": "oci://ghcr.io/super-phenix/charts/superphenix-api",
  "targetRevision": ""
}
</pre>
</td>
			<td>Superphenix API: backend serving the console and CLI, persists to PostgreSQL and delegates auth to Permify/Kratos.</td>
		</tr>
		<tr>
			<td>apps.superphenix-api.helm.values.config.database</td>
			<td>object</td>
			<td><pre lang="json">
{
  "database": "superphenix",
  "host": "postgres.{{ $.Release.Namespace }}.svc",
  "password": "{{ $.Values.apps.postgres.helm.values.auth.password | quote }}",
  "port": 5432,
  "username": "superphenix"
}
</pre>
</td>
			<td>Database storing users, organizations, projects and other API state.</td>
		</tr>
		<tr>
			<td>apps.superphenix-api.helm.values.config.permify</td>
			<td>object</td>
			<td><pre lang="json">
{
  "url": "permify.{{ $.Release.Namespace }}.svc:3478"
}
</pre>
</td>
			<td>Permify endpoint used for authorization checks.</td>
		</tr>
		<tr>
			<td>apps.superphenix-api.helm.values.domain</td>
			<td>string</td>
			<td><pre lang="json">
"api.superphenix.net"
</pre>
</td>
			<td>Public domain the API is exposed on.</td>
		</tr>
		<tr>
			<td>apps.superphenix-console</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "enabled": true,
    "prune": true,
    "selfHeal": true
  },
  "enabled": true,
  "helm": {
    "chart": "superphenix-console",
    "releaseName": "superphenix-console",
    "values": {
      "domain": "console.superphenix.net",
      "ingress": {
        "enabled": true,
        "hosts": [
          {
            "host": "console.superphenix.net",
            "paths": [
              {
                "path": "/"
              }
            ]
          }
        ],
        "tls": [
          {
            "hosts": [
              "console.superphenix.net"
            ],
            "secretName": "superphenix-console-tls"
          }
        ]
      }
    }
  },
  "modes": [
    "Management"
  ],
  "repoURL": "oci://ghcr.io/super-phenix/charts/superphenix-console",
  "targetRevision": "0.2.1"
}
</pre>
</td>
			<td>Superphenix Console: web UI exposed to end users.</td>
		</tr>
		<tr>
			<td>apps.superphenix-console.helm.values.domain</td>
			<td>string</td>
			<td><pre lang="json">
"console.superphenix.net"
</pre>
</td>
			<td>Public domain the console is exposed on. TODO: move ingress templating into the superphenix-console chart itself.</td>
		</tr>
		<tr>
			<td>apps.superphenix-controller</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": [
      "SkipDryRunOnMissingResource=true"
    ]
  },
  "enabled": true,
  "helm": {
    "chart": "superphenix-controller",
    "releaseName": "superphenix-controller",
    "values": {}
  },
  "modes": [
    "Hyperconverged",
    "DecoupledVirtualization"
  ],
  "namespace": "superphenix-system",
  "repoURL": "ghcr.io/super-phenix/charts",
  "targetRevision": "",
  "wave": "-10"
}
</pre>
</td>
			<td>Superphenix controller deployed on every availability-zone cluster.</td>
		</tr>
		<tr>
			<td>apps.talos-backup</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "talos-backup",
    "releaseName": "talos-backup"
  },
  "modes": [
    "Hyperconverged",
    "DecoupledStorage",
    "DecoupledVirtualization"
  ],
  "namespace": "talos-backup",
  "repoURL": "oci://ghcr.io/super-phenix/charts/talos-backup",
  "targetRevision": "0.1.0"
}
</pre>
</td>
			<td>talos-backup: CronJob that periodically pushes etcd snapshots to S3 for disaster recovery.</td>
		</tr>
		<tr>
			<td>apps.talos-operator</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "enabled": true,
    "prune": true,
    "selfHeal": true
  },
  "enabled": false,
  "helm": {
    "chart": "talos-operator",
    "releaseName": "talos-operator",
    "values": {
      "featureFlags": {
        "enablePxeBootStack": true
      }
    }
  },
  "modes": [
    "Management"
  ],
  "repoURL": "https://alperencelik.github.io/helm-charts",
  "targetRevision": "0.6.1"
}
</pre>
</td>
			<td>Talos Operator: manages bare-metal Talos nodes (PXE boot, machine configuration). Disabled by default.</td>
		</tr>
		<tr>
			<td>apps.traefik</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "traefik",
    "releaseName": "traefik",
    "values": {
      "accessLog": {
        "enabled": true
      },
      "api": {
        "dashboard": false
      },
      "deployment": {
        "enabled": true,
        "kind": "DaemonSet"
      },
      "gateway": {
        "enabled": false
      },
      "gatewayClass": {
        "enabled": true,
        "name": "traefik"
      },
      "global": {
        "checkNewVersion": false
      },
      "hostNetwork": true,
      "ingressClass": {
        "enabled": true
      },
      "podSecurityContext": {
        "runAsGroup": 0,
        "runAsNonRoot": false,
        "runAsUser": 0
      },
      "ports": {
        "kaas-api-tls": {
          "expose": {
            "default": true
          },
          "exposedPort": 7444,
          "http": {
            "tls": {
              "enabled": true
            }
          },
          "port": 7444,
          "protocol": "TCP"
        },
        "kaas-https": {
          "expose": {
            "default": true
          },
          "exposedPort": 7443,
          "http": {
            "tls": {
              "enabled": true
            }
          },
          "port": 7443,
          "protocol": "TCP"
        },
        "kaas-konnectivity": {
          "expose": {
            "default": true
          },
          "exposedPort": 7442,
          "http": {
            "tls": {
              "enabled": true
            }
          },
          "port": 7442,
          "protocol": "TCP"
        },
        "metrics": {
          "exposedPort": 9101,
          "port": 9101
        },
        "web": {
          "port": 80
        },
        "websecure": {
          "port": 443
        }
      },
      "providers": {
        "kubernetesGateway": {
          "enabled": true
        },
        "kubernetesIngressNGINX": {
          "enabled": true,
          "watchIngressWithoutClass": true
        }
      },
      "securityContext": {
        "capabilities": {
          "add": [
            "NET_BIND_SERVICE"
          ],
          "drop": [
            "ALL"
          ]
        }
      },
      "service": {
        "type": "ClusterIP"
      },
      "updateStrategy": {
        "rollingUpdate": {
          "maxSurge": 0,
          "maxUnavailable": 1
        },
        "type": "RollingUpdate"
      }
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledVirtualization"
  ],
  "namespace": "traefik-system",
  "nsLabels": {
    "pod-security.kubernetes.io/enforce": "privileged"
  },
  "repoURL": "https://traefik.github.io/charts",
  "targetRevision": "41.0.1",
  "wave": "0"
}
</pre>
</td>
			<td>Traefik: primary Ingress and Gateway API controller. Successor to ingress-nginx.</td>
		</tr>
		<tr>
			<td>apps.tuned</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": false,
  "helm": {
    "chart": "tuned",
    "releaseName": "tuned"
  },
  "modes": [
    "Hyperconverged",
    "DecoupledStorage",
    "DecoupledVirtualization"
  ],
  "namespace": "tuned-system",
  "nsLabels": {
    "pod-security.kubernetes.io/enforce": "privileged"
  },
  "repoURL": "oci://ghcr.io/super-phenix/charts/tuned",
  "targetRevision": "0.1.0",
  "wave": "0"
}
</pre>
</td>
			<td>TuneD: node-level performance-tuning daemon (Superphenix-specific profiles). Disabled by default.</td>
		</tr>
		<tr>
			<td>apps.velero</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "velero",
    "releaseName": "velero",
    "values": {
      "configuration": {
        "backupStorageLocation": [],
        "defaultItemOperationTimeout": "8h",
        "features": "EnableCSI",
        "namespace": "velero-system",
        "restoreResourcePriorities": "vpc.kubeovn.io,subnet.kubeovn.io,vpc-nat-gateway.kubeovn.io,iptables-eip.kubeovn.io,iptables-fip-rule.kubeovn.io,iptables-snat-rule.kubeovn.io,iptables-dnat-rule.kubeovn.io,switch-lb-rule.kubeovn.io,networkpolicy.networking.k8s.io,network-attachment-definition.k8s.cni.cncf.io,controllerrevision.apps,datauploads.velero.io,persistentvolume,persistentvolumeclaim,datavolume.cdi.kubevirt.io,secret,virtualmachine.kubevirt.io,cluster.cluster.x-k8s.io,kubevirtcluster.infrastructure.cluster.x-k8s.io,configmap,issuer.cert-manager.io,certificate.cert-manager.io,serviceaccount,role.rbac.authorization.k8s.io,rolebinding.rbac.authorization.k8s.io,clusterrole.rbac.authorization.k8s.io,clusterrolebinding.rbac.authorization.k8s.io,etcdmember.etcd-operator.cozystack.io,etcdcluster.etcd-operator.cozystack.io,etcdcluster.etcd.aenix.io,datastore.kamaji.clastix.io,gateway.gateway.networking.k8s.io,tlsroute.gateway.networking.k8s.io,deployment.apps,kamajicontrolplane.controlplane.cluster.x-k8s.io,kubeadmconfigtemplate.bootstrap.cluster.x-k8s.io,kubevirtmachinetemplate.infrastructure.cluster.x-k8s.io,kubeadmconfig.bootstrap.cluster.x-k8s.io,kubevirtmachine.infrastructure.cluster.x-k8s.io,machine.cluster.x-k8s.io,machineset.cluster.x-k8s.io,machinedeployment.cluster.x-k8s.io,mutatingadmissionpolicy.admissionregistration.k8s.io,mutatingadmissionpolicybinding.admissionregistration.k8s.io"
      },
      "credentials": {
        "secretContents": {
          "cloud": ""
        }
      },
      "deployNodeAgent": true,
      "extraObjects": [
        {
          "apiVersion": "v1",
          "data": {
            "node-agent-config.json": "{\n    \"loadConcurrency\": {\n        \"globalConfig\": 40\n    }\n}\n"
          },
          "kind": "ConfigMap",
          "metadata": {
            "name": "node-agent-config",
            "namespace": "velero-system"
          }
        }
      ],
      "initContainers": [
        {
          "image": "velero/velero-plugin-for-aws:v1.12.1",
          "imagePullPolicy": "IfNotPresent",
          "name": "velero-plugin-for-aws",
          "volumeMounts": [
            {
              "mountPath": "/target",
              "name": "plugins"
            }
          ]
        },
        {
          "image": "quay.io/kubevirt/kubevirt-velero-plugin:v0.8.0",
          "imagePullPolicy": "IfNotPresent",
          "name": "velero-plugin-for-kubevirt",
          "volumeMounts": [
            {
              "mountPath": "/target",
              "name": "plugins"
            }
          ]
        },
        {
          "image": "ghcr.io/super-phenix/superphenix-velero-plugin:v0.1.0",
          "imagePullPolicy": "Always",
          "name": "velero-plugin-for-superphenix",
          "volumeMounts": [
            {
              "mountPath": "/target",
              "name": "plugins"
            }
          ]
        }
      ],
      "metrics": {
        "serviceMonitor": {
          "enabled": true
        }
      },
      "nodeAgent": {
        "extraArgs": [
          "--node-agent-configmap=node-agent-config"
        ]
      },
      "snapshotsEnabled": false
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledWorkload"
  ],
  "namespace": "velero-system",
  "nsLabels": {
    "pod-security.kubernetes.io/enforce": "privileged"
  },
  "repoURL": "https://vmware-tanzu.github.io/helm-charts",
  "targetRevision": "12.0.0",
  "wave": "0"
}
</pre>
</td>
			<td>Velero: cluster backup and disaster-recovery across availability zones.</td>
		</tr>
		<tr>
			<td>apps.volume-replicator</td>
			<td>object</td>
			<td><pre lang="json">
{
  "automation": {
    "cleanupOnDeletion": false,
    "enabled": true,
    "prune": true,
    "selfHeal": true,
    "syncOptions": {}
  },
  "enabled": true,
  "helm": {
    "chart": "volume-replicator",
    "releaseName": "volume-replicator",
    "values": {
      "exclusionRegex": "^prime-.*$"
    }
  },
  "modes": [
    "Hyperconverged",
    "DecoupledVirtualization"
  ],
  "namespace": "volume-replicator",
  "repoURL": "ghcr.io/super-phenix/helm-charts",
  "targetRevision": "0.5.1"
}
</pre>
</td>
			<td>volume-replicator: Kubernetes controller that automates RBD mirroring between Ceph clusters (disaster-recovery).</td>
		</tr>
		<tr>
			<td>argocd</td>
			<td>object</td>
			<td><pre lang="json">
{
  "appVersion": "argoproj.io/v1alpha1",
  "namespace": "superphenix-system",
  "project": "default"
}
</pre>
</td>
			<td>Argo CD application parameters shared by every generated Application.</td>
		</tr>
		<tr>
			<td>argocd.appVersion</td>
			<td>string</td>
			<td><pre lang="json">
"argoproj.io/v1alpha1"
</pre>
</td>
			<td>Argo CD API version to use when generating Application resources.</td>
		</tr>
		<tr>
			<td>argocd.namespace</td>
			<td>string</td>
			<td><pre lang="json">
"superphenix-system"
</pre>
</td>
			<td>Namespace in which Argo CD Applications are created.</td>
		</tr>
		<tr>
			<td>argocd.project</td>
			<td>string</td>
			<td><pre lang="json">
"default"
</pre>
</td>
			<td>Argo CD project the generated Applications belong to.</td>
		</tr>
		<tr>
			<td>cleanupOnDeletion</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Add the Argo CD `resources-finalizer` to every Application so that resources are cascade-deleted when the Application is removed. Overrides any per-application `automation.cleanupOnDeletion` setting.</td>
		</tr>
		<tr>
			<td>cluster</td>
			<td>object</td>
			<td><pre lang="json">
{
  "availabilityZone": "",
  "deploymentTopology": "",
  "name": "in-cluster",
  "region": "",
  "type": ""
}
</pre>
</td>
			<td>Cluster on which the applications should be deployed. The pair (`deploymentTopology`, `type`) determines the "effective mode" used to filter the `apps` entries (see the `apps.*.modes` field).</td>
		</tr>
		<tr>
			<td>cluster.availabilityZone</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Availability zone the cluster belongs to (informational).</td>
		</tr>
		<tr>
			<td>cluster.deploymentTopology</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Deployment topology. One of: "Hyperconverged", "Decoupled".</td>
		</tr>
		<tr>
			<td>cluster.name</td>
			<td>string</td>
			<td><pre lang="json">
"in-cluster"
</pre>
</td>
			<td>Name of the target cluster. When set to "management" the Applications are deployed to the local Argo CD cluster ("in-cluster").</td>
		</tr>
		<tr>
			<td>cluster.region</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Geographical region the cluster belongs to (informational).</td>
		</tr>
		<tr>
			<td>cluster.type</td>
			<td>string</td>
			<td><pre lang="json">
""
</pre>
</td>
			<td>Cluster role. One of: "Storage", "Virtualization", "Management".</td>
		</tr>
		<tr>
			<td>disableAll</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Disable all applications, overriding any per-application `enabled` setting.</td>
		</tr>
		<tr>
			<td>forceManual</td>
			<td>bool</td>
			<td><pre lang="json">
false
</pre>
</td>
			<td>Force manual sync for all applications, overriding any per-application `automation.enabled` setting. Useful when troubleshooting or during upgrades.</td>
		</tr>
	</tbody>
</table>

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
