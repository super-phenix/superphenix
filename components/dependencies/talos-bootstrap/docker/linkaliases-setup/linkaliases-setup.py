#!/usr/bin/python3

# Injects LinkAliases via kernel command line arguments to all cluster nodes

import kubernetes as kube
import compression.zstd as zstd
import base64
import os
import yaml

CLUSTER_NAME = os.getenv("CLUSTER_NAME")
CLUSTER_NAMESPACE = os.getenv("CLUSTER_NAMESPACE")

def linkaliases_injection(node: dict) -> dict:
    """
    Injects LinkAliases into kernel cmdline arguments.

    Parameters
    ----------
    node: dict
        Node to inject LinkAliases to.

    Returns
    -------
    node: dict
        Modified node.
    """
    linkaliases_str = ""
    for p in node["configPatches"]:
        if "kind" in p.keys() and p["kind"] == "LinkAliasConfig":
            linkaliases_str += f"\n---\n{yaml.safe_dump(p)}"
    # Talos inline configurations specified in the kernel cmdline arguments need to be zstd compressed and encoded in base64:
    node["pxeClientSpec"]["kernelCmdlineArgs"] = f"talos.config.inline={base64.b64encode(zstd.compress(bytes(linkaliases_str, encoding = 'utf-8'))).decode('utf-8')} {node['pxeClientSpec']['kernelCmdlineArgs']}"

    return node

# Fetching TalosCluster resource:
kube.config.load_incluster_config()
custom_client = kube.client.CustomObjectsApi()
taloscluster : dict = custom_client.get_namespaced_custom_object(
    group = "talos.alperen.cloud",
    version = "v1alpha1",
    plural = "talosclusters",
    namespace = CLUSTER_NAMESPACE,
    name = CLUSTER_NAME
)

# Adding LinkAliases to kernel cmdline args:
for n in taloscluster["spec"]["controlPlane"]["metalSpec"]["machines"]:
    n = linkaliases_injection(n)
if "worker" in taloscluster["spec"].keys():
    for n in taloscluster["spec"]["worker"]["metalSpec"]["machines"]:
        n = linkaliases_injection(n)

# Patching TalosCluster with kernel cmdline arguments:
custom_client.patch_namespaced_custom_object(
    group = "talos.alperen.cloud",
    version = "v1alpha1",
    plural = "talosclusters",
    namespace = CLUSTER_NAMESPACE,
    name = CLUSTER_NAME,
    body = taloscluster
)