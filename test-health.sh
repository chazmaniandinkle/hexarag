#!/bin/bash

# Start the server in background
echo "Starting HexaRAG server..."
./bin/hexarag -config config.yaml &
SERVER_PID=$!

# Give server time to start
sleep 3

# Test health endpoint
echo "Testing health endpoint..."
HEALTH_RESPONSE=$(curl -s http://localhost:8080/health || echo "Failed to connect")

echo "Health endpoint response: $HEALTH_RESPONSE"

# Test API endpoint
echo "Testing conversations endpoint..."
CONVERSATIONS_RESPONSE=$(curl -s http://localhost:8080/api/v1/conversations || echo "Failed to connect")

echo "Conversations endpoint response: $CONVERSATIONS_RESPONSE"

# Stop the server
echo "Stopping server..."
kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true

echo "Test completed."