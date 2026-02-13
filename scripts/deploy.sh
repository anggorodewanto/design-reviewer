#!/bin/bash
set -e

echo "Deploying design-reviewer to Fly.io..."

# Create volume if first deploy
fly volumes list | grep -q "data" || fly volumes create data --region sin --size 5

# Deploy
fly deploy

echo "Deployed! URL: https://design-reviewer.fly.dev"
