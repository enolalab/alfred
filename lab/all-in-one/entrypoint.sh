#!/bin/sh

set -eu

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

k3s server --disable traefik --disable servicelb --write-kubeconfig-mode 644 &
K3S_PID=$!

/lab/scripts/wait-ready.sh
/bin/sh -c 'kubectl apply -f /lab/manifests/00-namespace.yaml && kubectl apply -f /lab/manifests/01-redis.yaml && kubectl apply -f /lab/manifests/02-mock-telegram-configmap.yaml && kubectl apply -f /lab/manifests/03-mock-telegram.yaml && kubectl apply -f /lab/manifests/04-sample-app.yaml && kubectl apply -f /lab/manifests/05-alfred-rbac.yaml'
/lab/scripts/render-runtime-manifests.sh
kubectl apply -f /tmp/alfred-lab-runtime
kubectl apply -f /lab/manifests/08-alfred.yaml
/lab/scripts/bootstrap.sh

wait "$K3S_PID"
