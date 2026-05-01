#!/bin/sh

# Substitute secrets into garage config (avoids hardcoding in the template)
sed "s|\${GARAGE_RPC_SECRET}|${GARAGE_RPC_SECRET}|g" /etc/garage.toml > /tmp/garage.toml

# Start Garage server in background
GARAGE_CONFIG_FILE=/tmp/garage.toml garage server &
GARAGE_PID=$!

echo "Waiting for Garage RPC to be available..."
until GARAGE_CONFIG_FILE=/tmp/garage.toml garage status > /dev/null 2>&1; do
  sleep 2
done

echo "Garage is up. Starting automatic initialization..."

# 1. Initialize Layout (for single-node dev setup)
NODE_ID=$(GARAGE_CONFIG_FILE=/tmp/garage.toml garage status | grep -v "====" | grep -v "ID" | grep -v "Hostname" | grep "[a-f0-9]" | awk '{print $1}' | head -n 1)

if [ -n "$NODE_ID" ] && [ "$NODE_ID" != "ID" ]; then
  echo "Found Node ID: $NODE_ID. Ensuring layout..."
  GARAGE_CONFIG_FILE=/tmp/garage.toml garage layout assign "$NODE_ID" --zone dev --capacity 100G --tag dev || echo "Layout assignment failed"
  GARAGE_CONFIG_FILE=/tmp/garage.toml garage layout apply --version 1 || echo "Layout application failed"
else
  echo "Warning: Could not find Node ID for layout assignment."
fi

# 2. Import Access Key
if [ -n "$STORAGE_ACCESS_KEY" ] && [ -n "$STORAGE_SECRET_KEY" ]; then
  echo "Importing Access Key: $STORAGE_ACCESS_KEY"
  GARAGE_CONFIG_FILE=/tmp/garage.toml garage key import "$STORAGE_ACCESS_KEY" "$STORAGE_SECRET_KEY" -n "omnigate-key" --yes || echo "Key import failed"
fi

# 3. Create Bucket
if [ -n "$STORAGE_BUCKET" ]; then
  echo "Creating Bucket: $STORAGE_BUCKET"
  GARAGE_CONFIG_FILE=/tmp/garage.toml garage bucket create "$STORAGE_BUCKET" || echo "Bucket creation failed"

  if [ -n "$STORAGE_ACCESS_KEY" ]; then
    echo "Granting permissions for $STORAGE_ACCESS_KEY on $STORAGE_BUCKET..."
    GARAGE_CONFIG_FILE=/tmp/garage.toml garage bucket allow "$STORAGE_BUCKET" --key "$STORAGE_ACCESS_KEY" --read --write || echo "Permission grant failed"
  fi

  echo "Enabling website mode for $STORAGE_BUCKET..."
  GARAGE_CONFIG_FILE=/tmp/garage.toml garage bucket website --allow "$STORAGE_BUCKET" || echo "Website mode enable failed"
fi

echo "Garage initialization complete. Server is running."

wait $GARAGE_PID
