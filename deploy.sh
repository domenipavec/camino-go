#!/bin/bash

set -e

TS="$(date +%Y-%m-%dT%H-%M-%S)"
IMAGE="eu.gcr.io/api-project-704693280880/camino"

docker pull golang:latest
docker build -t "$IMAGE:$TS" -t "$IMAGE:latest" .
docker push "$IMAGE:$TS"
docker push "$IMAGE:latest"

kubectl set image deployment/hribi "hribi=$IMAGE:$TS" --record
kubectl set image deployment/camino "camino=$IMAGE:$TS" --record
