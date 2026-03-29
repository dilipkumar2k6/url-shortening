FROM maven:3.8.4-openjdk-11-slim AS builder

WORKDIR /app
COPY pom.xml .
COPY src ./src
RUN mvn clean package -DskipTests

FROM flink:1.18.1-scala_2.12

# Copy the fat JAR into the user library directory
# Flink will automatically add files in usrlib to the classpath
COPY --from=builder /app/target/flink-analytics-1.0-SNAPSHOT.jar /opt/flink/usrlib/flink-analytics.jar
