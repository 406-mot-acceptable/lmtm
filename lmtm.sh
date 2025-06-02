#!/bin/bash

LOCAL_PORT=${1:-4431}
REMOTE_HOST=${2:-10.0.0.2}
REMOTE_PORT=${3:-443}
SSH_SERVER=${4:-102.217.231.190}
USERNAME=${5:-dato}

echo "Starting SSH tunnel..."
echo "Local port: $LOCAL_PORT"
echo "Remote host: $REMOTE_HOST:$REMOTE_PORT" 
echo "SSH server: $SSH_SERVER"
echo "Username: $USERNAME"
echo ""

ssh -oHostKeyAlgorithms=+ssh-rsa -L $LOCAL_PORT:$REMOTE_HOST:$REMOTE_PORT $USERNAME@$SSH_SERVER