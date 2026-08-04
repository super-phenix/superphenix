#!/usr/bin/python3

# Imports the given Talos cluster to talos-operator

import yaml
import argparse
import subprocess
import base64

# Arguments parsing
parser = argparse.ArgumentParser(prog = "Import existing cluster", description = "Import an existing Talos cluster into talos-operator")
# Name of the Kubernetes context where talos-operator is installed
parser.add_argument("context")
# Name of the Kubernetes namespace where talos-operator is installed
parser.add_argument("namespace")
# Name of the Talos cluster
parser.add_argument("cluster_name")
# Path to Talos secrets file
parser.add_argument("secrets_file")

args = parser.parse_args()

# Creating a Kubernetes secret for the control plane state that contains Talos secrets (will be read by the operator to import the cluster):
with open(args.secrets_file, "r") as secrets_file:
    secrets = secrets_file.read()
state_secret = yaml.safe_dump({
    "apiVersion": "v1",
    "kind": "Secret",
    "metadata": {
        "name": f"{args.cluster_name}-state",
        "labels": {"talos.alperen.cloud/type": "state"},
        "namespace": args.namespace
    },
    "type": "Opaque",
    "data": {
        "secretBundle": base64.b64encode(secrets.encode("utf-8")),
        "state": base64.b64encode("Ready".encode("utf-8"))
    }
})
subprocess.run(f"echo '{state_secret}' | kubectl apply --context {args.context} -f -", shell = True, check = True)