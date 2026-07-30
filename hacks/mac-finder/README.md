# Find MAC addresses and interfaces name of a cluster's nodes

`mac-finder` is a tool that lists all interfaces on a node, along with their MAC address, name and status. This is useful to fill talos-manager values.

## How to use

Fill the values of talos-manager for the cluster and save it to `values.yaml`. Then, run:
```bash
docker build -t mac-finder .
docker run --mount type=bind,source=./values.yaml,target=/values.yaml mac-finder
```