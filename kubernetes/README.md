# Upgrade Agent/Server Kubernetes Deployment

This directory contains Kubernetes manifests for deploying the upgrade agent and server components.

## Prerequisites

- A running Kubernetes cluster (such as Minikube)
- kubectl command-line tool configured to communicate with your cluster
- Docker images for upgrade-agent and upgrade-server built and available

## Build Docker Images

Before deploying to Kubernetes, ensure you have built the Docker images:

```bash
# From the root directory of the project
docker build -t upgrade-agent:latest .
docker build -t upgrade-server:latest -f Dockerfile.server .

# For Minikube, you need to load the images into Minikube's Docker daemon
minikube image load upgrade-agent:latest
minikube image load upgrade-server:latest
```

## Deploy to Kubernetes

### Labeling Nodes

Both the upgrade agent and server DaemonSets use a node selector to target nodes with the label `role=switch`. You'll need to label your nodes before deployment:

```bash
# Label your nodes that should run the upgrade components
kubectl label nodes <node-name> role=switch
```

For testing in Minikube, you can label the minikube node:

```bash
kubectl label nodes minikube role=switch
```

### Applying Manifests

Apply the manifests in the following order:

```bash
# Deploy the upgrade server
kubectl apply -f kubernetes/upgrade-server-daemonset.yaml
kubectl apply -f kubernetes/upgrade-server-service.yaml

# Deploy the upgrade agent configuration
kubectl apply -f kubernetes/upgrade-agent-config.yaml

# Deploy the upgrade agent
kubectl apply -f kubernetes/upgrade-agent-daemonset.yaml
```

By default, the upgrade server is configured to use "fake reboot" mode, which simulates reboots without actually restarting the system. This is useful for testing the upgrade process without disrupting your environment.

## Verify Deployment

Check if the DaemonSets are running correctly:

```bash
kubectl get daemonsets
kubectl get pods
kubectl get services
```

## Updating Firmware Version

To trigger a firmware update, update the ConfigMap with a new target version:

```bash
kubectl edit configmap upgrade-agent-config
```

Change the `targetVersion` field to the new desired version (e.g., "1.1.0").

## Viewing Logs

To view logs from the upgrade server or agent:

```bash
# Get pod names
kubectl get pods

# View logs for a specific pod
kubectl logs <pod-name>
```

## Cleaning Up

To remove all deployed resources:

```bash
kubectl delete -f kubernetes/upgrade-agent-daemonset.yaml
kubectl delete -f kubernetes/upgrade-agent-config.yaml
kubectl delete -f kubernetes/upgrade-server-service.yaml
kubectl delete -f kubernetes/upgrade-server-daemonset.yaml
```
