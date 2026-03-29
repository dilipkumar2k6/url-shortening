#!/bin/bash

set -e

CLUSTER_NAME="url-shortener"
FLINK_MODE="sql"

# Parse arguments
for i in "$@"; do
    case $i in
        --flink=*)
            FLINK_MODE="${i#*=}"
            shift
            ;;
        *)
            ;;
    esac
done

echo "Using Flink mode: $FLINK_MODE"

# 1. Create kind cluster if not exists
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo "Creating kind cluster..."
    kind create cluster --name ${CLUSTER_NAME}
else
    echo "Kind cluster ${CLUSTER_NAME} already exists."
fi

echo "Exporting kubeconfig for kind cluster..."
KUBECONFIG_FILE="$HOME/.kube/kind-${CLUSTER_NAME}"
kind get kubeconfig --name ${CLUSTER_NAME} > "$KUBECONFIG_FILE"
export KUBECONFIG="$KUBECONFIG_FILE"

# 1.1 Install Istio
echo "Installing Istio..."
if ! command -v istioctl &> /dev/null; then
    curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.21.0 TARGET_ARCH=x86_64 sh -
    export PATH=$PATH:$(pwd)/istio-1.21.0/bin
fi
istioctl install --set profile=demo -y
kubectl label namespace default istio-injection=enabled --overwrite

# 2. Build Docker images
echo "Building Docker images..."
if [ -f .env ]; then
    echo "Sourcing .env file..."
    export $(grep -v '^#' .env | xargs)
fi

docker build -t write-api:latest -f backend/write-service/cmd/write-api/Dockerfile .
docker build -t cdc-worker:latest -f backend/write-service/cmd/cdc-worker/Dockerfile .
docker build -t read-api:latest -f backend/read-service/cmd/read-api/Dockerfile .
docker build -t analytics-api:latest -f backend/analytics-service/cmd/analytics-api/Dockerfile .

if [ "$FLINK_MODE" = "java" ]; then
    echo "Building Flink Java image..."
    docker build -t flink-custom:latest -f backend/analytics-service/flink-java/Dockerfile backend/analytics-service/flink-java/
else
    echo "Building Flink SQL image..."
    docker build -t flink-custom:latest -f backend/analytics-service/flink/Dockerfile backend/analytics-service/flink/
fi

docker build -t frontend:latest \
    --build-arg VITE_SHORT_LINK_BASE_URL=http://localhost:10001 \
    --build-arg VITE_FIREBASE_API_KEY=$VITE_FIREBASE_API_KEY \
    --build-arg VITE_FIREBASE_AUTH_DOMAIN=$VITE_FIREBASE_AUTH_DOMAIN \
    --build-arg VITE_FIREBASE_PROJECT_ID=$VITE_FIREBASE_PROJECT_ID \
    --build-arg VITE_FIREBASE_STORAGE_BUCKET=$VITE_FIREBASE_STORAGE_BUCKET \
    --build-arg VITE_FIREBASE_MESSAGING_SENDER_ID=$VITE_FIREBASE_MESSAGING_SENDER_ID \
    --build-arg VITE_FIREBASE_APP_ID=$VITE_FIREBASE_APP_ID \
    --build-arg VITE_USE_AUTH_EMULATOR=$VITE_USE_AUTH_EMULATOR \
    -f frontend/Dockerfile .

# 3. Load images into kind
echo "Loading images into kind..."
kind load docker-image write-api:latest --name ${CLUSTER_NAME}
kind load docker-image cdc-worker:latest --name ${CLUSTER_NAME}
kind load docker-image read-api:latest --name ${CLUSTER_NAME}
kind load docker-image analytics-api:latest --name ${CLUSTER_NAME}
kind load docker-image flink-custom:latest --name ${CLUSTER_NAME}
kind load docker-image frontend:latest --name ${CLUSTER_NAME}

# 4. Deploy Infrastructure
echo "Deploying infrastructure..."
kubectl apply -f k8s/infra/

# 5. Wait for Infrastructure
echo "Waiting for infrastructure to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/spanner-emulator
kubectl wait --for=condition=available --timeout=300s deployment/etcd
kubectl wait --for=condition=available --timeout=300s deployment/redis
kubectl wait --for=condition=available --timeout=300s deployment/redis-ratelimit
kubectl wait --for=condition=available --timeout=300s deployment/kafka
kubectl wait --for=condition=available --timeout=300s deployment/clickhouse
kubectl wait --for=condition=available --timeout=300s deployment/ratelimit
 
# 5.1 Deploy SigNoz Infrastructure
echo "Deploying SigNoz Infrastructure..."
kubectl apply -f k8s/signoz/signoz-standalone.yaml

echo "Waiting for ZooKeeper..."
kubectl wait --for=condition=available --timeout=300s deployment/signoz-zookeeper
# Critical: Wait for ZooKeeper to accept connections
sleep 30

echo "Waiting for ClickHouse..."
kubectl wait --for=condition=available --timeout=300s deployment/signoz-clickhouse
# Give ClickHouse time to connect to ZooKeeper and form the 'cluster'
sleep 30

echo "Initializing SigNoz ClickHouse Databases..."
kubectl exec deployment/signoz-clickhouse -c clickhouse -- clickhouse-client --query "CREATE DATABASE IF NOT EXISTS signoz_traces"
kubectl exec deployment/signoz-clickhouse -c clickhouse -- clickhouse-client --query "CREATE DATABASE IF NOT EXISTS signoz_metrics"
kubectl exec deployment/signoz-clickhouse -c clickhouse -- clickhouse-client --query "CREATE DATABASE IF NOT EXISTS signoz_logs"
kubectl exec deployment/signoz-clickhouse -c clickhouse -- clickhouse-client --query "CREATE DATABASE IF NOT EXISTS signoz_metadata"
kubectl exec deployment/signoz-clickhouse -c clickhouse -- clickhouse-client --query "CREATE DATABASE IF NOT EXISTS signoz_analytics"
kubectl exec deployment/signoz-clickhouse -c clickhouse -- clickhouse-client --query "CREATE DATABASE IF NOT EXISTS signoz_meter"

# 5.2 Deploy SigNoz (rest of components)
echo "Deploying SigNoz..."
kubectl apply -f k8s/signoz/
echo "Deleting SigNoz Query Service pod to clear ephemeral metadata DB..."
kubectl delete pod -l app=signoz-query-service --ignore-not-found
echo "Waiting for NEW SigNoz Query Service pod to be ready..."
sleep 5
kubectl wait --for=condition=ready --timeout=300s pod -l app=signoz-query-service
echo "Waiting for SigNoz components to be available..."
kubectl wait --for=condition=available --timeout=300s deployment/signoz-clickhouse
kubectl wait --for=condition=available --timeout=300s deployment/signoz-query-service
kubectl wait --for=condition=available --timeout=300s deployment/signoz-frontend
kubectl wait --for=condition=available --timeout=300s deployment/signoz-otel-collector
kubectl wait --for=condition=available --timeout=300s deployment/signoz-zookeeper

# Wait for ZooKeeper to be fully ready for connections
echo "Waiting 30s for ZooKeeper to be ready for connections..."
sleep 30

# 5.3 Initialize SigNoz ClickHouse Schema (Migrations)
echo "Initializing SigNoz ClickHouse Schema..."
kubectl wait --for=condition=ready --timeout=300s pod -l app=signoz-otel-collector
echo "Running SigNoz Migrations..."
kubectl exec deployment/signoz-otel-collector -- env SIGNOZ_OTEL_COLLECTOR_CLICKHOUSE_DSN=tcp://signoz-clickhouse:9000 /signoz-otel-collector migrate bootstrap --clickhouse-cluster "cluster" --clickhouse-replication=false
kubectl exec deployment/signoz-otel-collector -- env SIGNOZ_OTEL_COLLECTOR_CLICKHOUSE_DSN=tcp://signoz-clickhouse:9000 /signoz-otel-collector migrate sync up --clickhouse-cluster "cluster" --clickhouse-replication=false
kubectl exec deployment/signoz-otel-collector -- env SIGNOZ_OTEL_COLLECTOR_CLICKHOUSE_DSN=tcp://signoz-clickhouse:9000 /signoz-otel-collector migrate async up --clickhouse-cluster "cluster" --clickhouse-replication=false

# 5.4 Initialize Analytics ClickHouse Schema
echo "Initializing Analytics ClickHouse Schema..."
kubectl exec deployment/clickhouse -- clickhouse-client --query "$(cat backend/analytics-service/clickhouse.sql)"

# 5.5 Initialize Kafka Topics
echo "Initializing Kafka Topics..."
kubectl exec deployment/kafka -- /opt/kafka/bin/kafka-topics.sh --delete --bootstrap-server localhost:9092 --topic url-created --if-exists
kubectl exec deployment/kafka -- /opt/kafka/bin/kafka-topics.sh --delete --bootstrap-server localhost:9092 --topic click-events --if-exists
kubectl exec deployment/kafka -- /opt/kafka/bin/kafka-topics.sh --create --bootstrap-server localhost:9092 --replication-factor 1 --partitions 1 --topic url-created --if-not-exists
kubectl exec deployment/kafka -- /opt/kafka/bin/kafka-topics.sh --create --bootstrap-server localhost:9092 --replication-factor 1 --partitions 1 --topic click-events --if-not-exists

# 6. Initialize Spanner Emulator
echo "Initializing Spanner Emulator..."
kubectl rollout restart deployment/spanner-emulator
kubectl wait --for=condition=available --timeout=300s deployment/spanner-emulator
# Wait a bit for the emulator to be actually ready to accept connections
sleep 10
kubectl delete pod spanner-init --ignore-not-found
kubectl run spanner-init --image=curlimages/curl --restart=Never \
  --annotations="sidecar.istio.io/inject=false" -- /bin/sh -c \
  "curl -X POST http://spanner-emulator:9020/v1/projects/url-shortener/instances -d '{\"instanceId\": \"main\", \"instance\": {\"config\": \"projects/url-shortener/instanceConfigs/emulator-config\", \"displayName\": \"Main Instance\", \"nodeCount\": 1}}' && \
   curl -X POST http://spanner-emulator:9020/v1/projects/url-shortener/instances/main/databases -d '{\"createStatement\": \"CREATE DATABASE urls\"}' && \
   curl -X PATCH http://spanner-emulator:9020/v1/projects/url-shortener/instances/main/databases/urls/ddl -d '{\"statements\": [\"CREATE TABLE url_mappings (id STRING(MAX) NOT NULL, long_url STRING(MAX) NOT NULL, user_id STRING(MAX), created_at TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=true)) PRIMARY KEY (id)\"]}'"

# 7. Initialize ClickHouse Schema
echo "Initializing ClickHouse Schema..."
# Wait a bit for ClickHouse to be fully ready to accept connections
sleep 10
kubectl exec deployment/clickhouse -- clickhouse-client --query "$(cat backend/analytics-service/clickhouse.sql)"

# 8. Deploy Application
echo "Deploying application..."
kubectl apply -f k8s/istio/
kubectl apply -R -f k8s/write-api/
kubectl apply -R -f k8s/cdc-worker/
kubectl apply -R -f k8s/read-api/
kubectl apply -R -f k8s/analytics-api/
kubectl apply -f k8s/flink/
kubectl apply -f k8s/frontend.yaml

# 11. Restart deployments to pick up new images/configs
echo "Restarting deployments to pick up new images..."
kubectl rollout restart deployment/write-api
kubectl rollout restart deployment/cdc-worker
kubectl rollout restart deployment/read-api
kubectl rollout restart deployment/analytics-api
kubectl rollout restart deployment/flink-jobmanager
kubectl rollout restart deployment/flink-taskmanager
# kubectl rollout restart deployment/envoy # Replaced by Istio
kubectl rollout restart deployment/frontend
kubectl rollout restart deployment/signoz-query-service

# 12. Wait for application to be ready
echo "Waiting for application to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/istio-ingressgateway -n istio-system
# kubectl wait --for=condition=available --timeout=300s deployment/envoy
# kubectl wait --for=condition=available --timeout=300s deployment/envoy-read
kubectl wait --for=condition=available --timeout=300s deployment/write-api
kubectl wait --for=condition=available --timeout=300s deployment/cdc-worker
kubectl wait --for=condition=available --timeout=300s deployment/read-api
kubectl wait --for=condition=available --timeout=300s deployment/analytics-api
kubectl wait --for=condition=available --timeout=300s deployment/flink-jobmanager
kubectl wait --for=condition=available --timeout=300s deployment/flink-taskmanager
kubectl wait --for=condition=available --timeout=300s deployment/frontend

# 12.1 Submit Flink Job
echo "Submitting Flink Job ($FLINK_MODE mode)..."
# Wait for JobManager to be fully ready to accept jobs
echo "Waiting for Flink JobManager to be ready to accept jobs..."
for i in {1..30}; do
    if kubectl exec deployment/flink-jobmanager -- curl -s http://localhost:8081/overview > /dev/null 2>&1; then
        echo "Flink JobManager is ready!"
        break
    fi
    echo "Waiting for Flink JobManager... ($i/30)"
    sleep 2
done

if [ "$FLINK_MODE" = "java" ]; then
    echo "Running Flink Java Job..."
    kubectl exec deployment/flink-jobmanager -- ./bin/flink run -d /opt/flink/usrlib/flink-analytics.jar
else
    echo "Running Flink SQL Job..."
    # Substitute environment variables in the SQL script before running
    KAFKA_ADDR=$(kubectl exec deployment/flink-jobmanager -- printenv KAFKA_BOOTSTRAP_SERVERS)
    CH_URL=$(kubectl exec deployment/flink-jobmanager -- printenv CLICKHOUSE_URL)
    CH_USER=$(kubectl exec deployment/flink-jobmanager -- printenv CLICKHOUSE_USER)
    CH_PWD=$(kubectl exec deployment/flink-jobmanager -- printenv CLICKHOUSE_PASSWORD)
    
    kubectl exec deployment/flink-jobmanager -- sed -i "s|\${KAFKA_BOOTSTRAP_SERVERS}|$KAFKA_ADDR|g" /opt/flink/click_events_to_clickhouse.sql
    kubectl exec deployment/flink-jobmanager -- sed -i "s|\${CLICKHOUSE_URL}|$CH_URL|g" /opt/flink/click_events_to_clickhouse.sql
    kubectl exec deployment/flink-jobmanager -- sed -i "s|\${CLICKHOUSE_USER}|$CH_USER|g" /opt/flink/click_events_to_clickhouse.sql
    kubectl exec deployment/flink-jobmanager -- sed -i "s|\${CLICKHOUSE_PASSWORD}|$CH_PWD|g" /opt/flink/click_events_to_clickhouse.sql

    kubectl exec deployment/flink-jobmanager -- ./bin/flink run -d -c org.apache.flink.table.client.SqlClient /opt/flink/lib/flink-sql-client-*.jar -f /opt/flink/click_events_to_clickhouse.sql || \
    kubectl exec deployment/flink-jobmanager -- ./bin/sql-client.sh -f /opt/flink/click_events_to_clickhouse.sql
fi

echo "Setup complete!"

# 13. Port-forward Envoy in background
echo "Exposing services via port-forward..."
# Kill existing port-forwards if any
pkill -f "port-forward svc/envoy" || true
pkill -f "port-forward svc/envoy-read" || true
pkill -f "port-forward svc/signoz-frontend" || true

kubectl port-forward svc/envoy -n istio-system 10000:80 > /dev/null 2>&1 &
kubectl port-forward svc/envoy-read -n istio-system 10001:80 > /dev/null 2>&1 &
kubectl port-forward svc/signoz-frontend 3301:3301 > /dev/null 2>&1 &

echo "--------------------------------------------------"
echo "HyperShort is now accessible at:"
echo "Frontend & Write API: http://localhost:10000"
echo "Read API (Redirects): http://localhost:10001"
echo ""
echo "Monitoring:"
echo "SigNoz Dashboard: http://localhost:3301"
echo "--------------------------------------------------"
echo "To test, run: ./test.sh"
