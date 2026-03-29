#!/bin/bash

set -e

CLUSTER_NAME="url-shortener"
KUBECONFIG_FILE="$HOME/.kube/kind-${CLUSTER_NAME}"

if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo "Exporting kubeconfig for kind cluster..."
    kind get kubeconfig --name ${CLUSTER_NAME} > "$KUBECONFIG_FILE"
    export KUBECONFIG="$KUBECONFIG_FILE"
else
    echo "Kind cluster ${CLUSTER_NAME} does not exist."
    exit 1
fi

# 1. Port-forward Envoy in background
echo "Port-forwarding Envoy..."
kubectl port-forward svc/envoy -n istio-system 10000:80 > /dev/null 2>&1 &
PF_PID=$!

# 1.1 Port-forward SigNoz Frontend
echo "Port-forwarding SigNoz Frontend..."
kubectl port-forward svc/signoz-frontend 3301:3301 > /dev/null 2>&1 &
PF_SIGNOZ_PID=$!

# Wait for port-forward to be ready
echo "Waiting for port-forward to be ready..."
for i in {1..10}; do
    if curl -s http://localhost:10000/health > /dev/null; then
        echo "Port-forward ready!"
        break
    fi
    sleep 1
done

# Cleanup on exit
trap "kill $PF_PID 2>/dev/null || true" EXIT

UNIQUE_URL="https://example.com/url-$(date +%s)-$RANDOM"
echo "Testing URL shortening via Envoy with URL: $UNIQUE_URL"
RESPONSE=$(curl -s -X POST http://localhost:10000/api/v1/shorten \
    -H "Content-Type: application/json" \
    -d "{\"long_url\": \"$UNIQUE_URL\"}")

echo "Response: $RESPONSE"

if echo "$RESPONSE" | grep -q "short_url"; then
    echo "SUCCESS: Received short URL"
else
    echo "FAILURE: Did not receive short URL"
    exit 1
fi

# 3. Test Rate Limiting (Write API)
echo "Testing Rate Limiting for Write API (sending 15 requests)..."
for i in {1..15}; do
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:10000/api/v1/shorten \
        -H "Content-Type: application/json" \
        -d '{"long_url": "https://example.com/rate/limit/test"}')
    echo "Request $i: Status $STATUS"
done

# 4. Port-forward Write API directly for logged-in user tests (bypassing Envoy JWT)
echo "Port-forwarding Write API..."
kubectl port-forward deployment/write-api 10002:8080 > /dev/null 2>&1 &
PF_WRITE_PID=$!

# Wait for port-forward to be ready
echo "Waiting for port-forward write to be ready..."
for i in {1..10}; do
    if curl -s http://localhost:10000/health > /dev/null; then
        echo "Port-forward write ready!"
        break
    fi
    sleep 1
done

# 4.1 Port-forward Analytics API directly for history tests
echo "Port-forwarding Analytics API..."
kubectl port-forward deployment/analytics-api 10003:8080 > /dev/null 2>&1 &
PF_ANALYTICS_PID=$!

# Wait for port-forward to be ready
echo "Waiting for port-forward analytics to be ready..."
for i in {1..20}; do
    if curl -s http://localhost:10000/health > /dev/null; then
        echo "Port-forward analytics ready!"
        break
    fi
    sleep 1
done

# 5. Port-forward Envoy Read in background
echo "Port-forwarding Envoy Read..."
kubectl port-forward svc/envoy-read -n istio-system 10001:80 > /dev/null 2>&1 &
PF_READ_PID=$!

# Wait for port-forward to be ready
echo "Waiting for port-forward read to be ready..."
for i in {1..10}; do
    if curl -s http://localhost:10001/health > /dev/null; then
        echo "Port-forward read ready!"
        break
    fi
    sleep 1
done

# Cleanup on exit
trap "kill $PF_PID $PF_READ_PID $PF_WRITE_PID $PF_ANALYTICS_PID $PF_SIGNOZ_PID 2>/dev/null || true" EXIT

# 5. Test Redirect via Envoy Read
echo "Testing URL redirect via Envoy Read..."
# Use the short code from the first request (extract from the end of the URL)
SHORT_CODE=$(echo "$RESPONSE" | grep -oP '(?<="short_url":")[^"]+' | sed 's/.*\///')
echo "Short Code: $SHORT_CODE"

REDIRECT_RESPONSE=$(curl -s -I http://localhost:10001/$SHORT_CODE)
echo "$REDIRECT_RESPONSE" | grep -q "HTTP/1.1 302" && echo "SUCCESS: Received 302 Redirect" || echo "FAILURE: Did not receive 302 Redirect"

# 7. Test Edit and Delete Features (Logged-in User)
echo "Testing Edit and Delete features for logged-in user..."
LOGGED_IN_USER="test-user-$(date +%s)"
CUSTOM_SLUG="edit-test-$(date +%s)"

echo "1. Shortening URL with custom slug: $CUSTOM_SLUG"
SHORTEN_RESPONSE=$(curl -s -X POST http://localhost:10000/api/v1/shorten \
    -H "Content-Type: application/json" \
    -H "X-User-Id: $LOGGED_IN_USER" \
    -d "{\"long_url\": \"https://www.google.com\", \"custom_slug\": \"$CUSTOM_SLUG\"}")

if echo "$SHORTEN_RESPONSE" | grep -q "$CUSTOM_SLUG"; then
    echo "SUCCESS: Created link with custom slug"
else
    echo "FAILURE: Failed to create link with custom slug. Response: $SHORTEN_RESPONSE"
    exit 1
fi

echo "1b. Generating clicks for $CUSTOM_SLUG..."
for i in {1..5}; do
    curl -s -o /dev/null http://localhost:10001/$CUSTOM_SLUG
done
echo "Waiting for analytics aggregation (30s)..."
sleep 30

echo "1c. Verifying link in top performant..."
TOP_LINKS=$(curl -s http://localhost:10000/api/v1/analytics/top)
if echo "$TOP_LINKS" | grep -q "$CUSTOM_SLUG"; then
    echo "SUCCESS: Link found in top performant"
else
    echo "FAILURE: Link not found in top performant. Response: $TOP_LINKS"
    exit 1
fi

echo "2. Verifying link in history..."
HISTORY_RESPONSE=$(curl -s -X GET http://localhost:10000/api/v1/user/history -H "X-User-Id: $LOGGED_IN_USER")
if echo "$HISTORY_RESPONSE" | grep -q "$CUSTOM_SLUG"; then
    echo "SUCCESS: Link found in history"
else
    echo "FAILURE: Link not found in history. Response: $HISTORY_RESPONSE"
    exit 1
fi

echo "3. Updating destination URL..."
UPDATE_RESPONSE=$(curl -s -X PATCH http://localhost:10000/api/v1/links/$CUSTOM_SLUG \
    -H "Content-Type: application/json" \
    -H "X-User-Id: $LOGGED_IN_USER" \
    -d '{"long_url": "https://www.bing.com"}')

if echo "$UPDATE_RESPONSE" | grep -q "successfully"; then
    echo "SUCCESS: URL updated successfully"
else
    echo "FAILURE: Failed to update URL. Response: $UPDATE_RESPONSE"
    exit 1
fi

echo "3b. Verifying redirect to new URL..."
REDIRECT_RESPONSE_NEW=$(curl -s -I http://localhost:10001/$CUSTOM_SLUG)
if echo "$REDIRECT_RESPONSE_NEW" | grep -qi "Location: https://www.bing.com"; then
    echo "SUCCESS: Redirected to new URL"
else
    echo "FAILURE: Did not redirect to new URL. Response: $REDIRECT_RESPONSE_NEW"
    exit 1
fi

echo "4. Verifying update in history..."
HISTORY_RESPONSE=$(curl -s -X GET http://localhost:10000/api/v1/user/history -H "X-User-Id: $LOGGED_IN_USER")
if echo "$HISTORY_RESPONSE" | grep -q "bing.com"; then
    echo "SUCCESS: Updated URL found in history"
else
    echo "FAILURE: Updated URL not found in history. Response: $HISTORY_RESPONSE"
    exit 1
fi

echo "5. Deleting link..."
DELETE_RESPONSE=$(curl -s -X DELETE http://localhost:10000/api/v1/links/$CUSTOM_SLUG \
    -H "X-User-Id: $LOGGED_IN_USER")

if echo "$DELETE_RESPONSE" | grep -q "successfully"; then
    echo "SUCCESS: Link deleted successfully"
else
    echo "FAILURE: Failed to delete link. Response: $DELETE_RESPONSE"
    exit 1
fi

echo "5b. Verifying deletion in top performant..."
TOP_LINKS_AFTER=$(curl -s http://localhost:10000/api/v1/analytics/top)
if ! echo "$TOP_LINKS_AFTER" | grep -q "$CUSTOM_SLUG"; then
    echo "SUCCESS: Link removed from top performant"
else
    echo "FAILURE: Link still found in top performant after deletion. Response: $TOP_LINKS_AFTER"
    exit 1
fi

echo "6. Verifying deletion in history..."
HISTORY_RESPONSE=$(curl -s -X GET http://localhost:10000/api/v1/user/history -H "X-User-Id: $LOGGED_IN_USER")
if ! echo "$HISTORY_RESPONSE" | grep -q "$CUSTOM_SLUG"; then
    echo "SUCCESS: Link removed from history"
else
    echo "FAILURE: Link still found in history after deletion. Response: $HISTORY_RESPONSE"
    exit 1
fi

# 7. Verify storage
echo "Verifying storage..."

echo "Checking Redis for cache warming..."
# The short code should be in Redis
REDIS_VAL=$(kubectl exec deployment/redis -- redis-cli GET $SHORT_CODE)
if [ "$REDIS_VAL" != "" ]; then
    echo "SUCCESS: Found short URL in Redis: $REDIS_VAL"
else
    echo "FAILURE: Short URL not found in Redis"
fi

echo "Checking Kafka for events..."
# Get current offset to only read new messages
OFFSET=$(kubectl exec deployment/kafka -- /opt/kafka/bin/kafka-run-class.sh org.apache.kafka.tools.GetOffsetShell --bootstrap-server localhost:9092 --topic url-created --time -1 2>/dev/null | grep "url-created:0:" | cut -d: -f3)
if [ -z "$OFFSET" ]; then OFFSET=0; fi

# Perform one more request to ensure we catch an event after the offset
echo "Sending one more request to verify Kafka event..."
CURL_OUT=$(curl -s -X POST http://localhost:10000/api/v1/shorten -H "Content-Type: application/json" -d "{\"long_url\": \"https://example.com/kafka/test/$(date +%s)\"}")
# Use the short code from the first request (extract from the end of the URL)
NEW_SHORT_CODE=$(echo "$CURL_OUT" | grep -oP '(?<="short_url":")[^"]+' | sed 's/.*\///')

# Read messages from OFFSET
kubectl exec deployment/kafka -- /opt/kafka/bin/kafka-console-consumer.sh --bootstrap-server localhost:9092 --topic url-created --partition 0 --offset $OFFSET --max-messages 20 --timeout-ms 10000 > kafka_output.txt 2>/dev/null || true

if grep -q "$NEW_SHORT_CODE" kafka_output.txt; then
    echo "SUCCESS: Found short code in Kafka: $NEW_SHORT_CODE"
else
    echo "FAILURE: Short code $NEW_SHORT_CODE not found in Kafka"
    echo "Messages read from offset $OFFSET:"
    cat kafka_output.txt
fi
rm -f kafka_output.txt

echo "Verifying SigNoz Telemetry..."
# 8. Verify SigNoz Telemetry
echo "Checking ClickHouse for traces..."
# Wait a bit for telemetry to be exported and ingested
sleep 10
TRACE_COUNT=$(kubectl exec deployment/signoz-clickhouse -- clickhouse-client --query "SELECT count() FROM signoz_traces.signoz_index_v2" 2>/dev/null || echo "0")
if [ "$TRACE_COUNT" -gt 0 ]; then
    echo "SUCCESS: Found $TRACE_COUNT traces in ClickHouse"
else
    echo "WARNING: No traces found in ClickHouse yet"
fi

echo "Checking ClickHouse for metrics..."
METRIC_COUNT=$(kubectl exec deployment/signoz-clickhouse -- clickhouse-client --query "SELECT count() FROM signoz_metrics.samples_v4" 2>/dev/null || echo "0")
if [ "$METRIC_COUNT" -gt 0 ]; then
    echo "SUCCESS: Found $METRIC_COUNT metrics in ClickHouse"
else
    echo "WARNING: No metrics found in ClickHouse yet"
fi

echo "Checking ClickHouse for logs..."
LOG_COUNT=$(kubectl exec deployment/signoz-clickhouse -- clickhouse-client --query "SELECT count() FROM signoz_logs.logs_v2" 2>/dev/null || echo "0")
if [ "$LOG_COUNT" -gt 0 ]; then
    echo "SUCCESS: Found $LOG_COUNT logs in ClickHouse"
else
    echo "WARNING: No logs found in ClickHouse yet"
fi

echo "Verifying Flink Job Status..."
FLINK_JOBS=$(kubectl exec deployment/flink-jobmanager -- curl -s http://localhost:8081/jobs)
if echo "$FLINK_JOBS" | grep -q "RUNNING"; then
    JOB_NAME=$(echo "$FLINK_JOBS" | grep -o '"name":"[^"]*"' | head -1 | cut -d'"' -f4)
    echo "SUCCESS: Flink job '$JOB_NAME' is RUNNING"
else
    echo "FAILURE: Flink job is not running"
    echo "Flink Jobs: $FLINK_JOBS"
    exit 1
fi

# 9. Run Playwright Integration Tests
echo "Deleting SigNoz Query Service pod to clear ephemeral DB..."
kubectl delete pod -l app=signoz-query-service
kubectl wait --for=condition=ready --timeout=300s pod -l app=signoz-query-service
sleep 10

echo "Waiting for SigNoz ClickHouse schema to be initialized..."
for i in {1..30}; do
    DB_EXISTS=$(kubectl exec deployment/signoz-clickhouse -- clickhouse-client --query "SHOW DATABASES" | grep signoz_metrics || true)
    if [ "$DB_EXISTS" != "" ]; then
        echo "SigNoz ClickHouse schema is ready!"
        break
    fi
    echo "Waiting for signoz_metrics database... ($i/30)"
    sleep 5
done

echo "Waiting for analytics aggregation..."
sleep 15
echo "Running SigNoz UI Verification Tests..."
cd frontend
# Run only the signoz spec
corepack npm test tests/signoz.spec.js
cd ..

# Preserve screenshots
echo "Preserving screenshots..."
mkdir -p captured_screenshots
cp frontend/test-results/signoz-ui-metric-*.png captured_screenshots/ 2>/dev/null || echo "No screenshots found to copy."

echo "Verification complete! Screenshots available in: ./captured_screenshots"
