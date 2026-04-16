#!/bin/sh

set -eu

kubectl rollout status deployment/redis -n alfred-lab --timeout=180s
kubectl rollout status deployment/mock-telegram-api -n alfred-lab --timeout=180s
kubectl rollout status deployment/payments-api -n alfred-lab --timeout=180s
kubectl rollout status deployment/alfred -n alfred-lab --timeout=180s

echo "Alfred all-in-one lab bootstrapped."
echo "Namespace: alfred-lab"
echo "Use kubectl get pods -n alfred-lab to inspect state."
