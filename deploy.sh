#!/bin/bash
set -e
docker compose build
docker compose up -d --force-recreate
echo "Deploy complete"
