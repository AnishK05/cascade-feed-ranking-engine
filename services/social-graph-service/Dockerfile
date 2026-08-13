FROM eclipse-temurin:21-jdk-jammy AS build
WORKDIR /src
COPY . .
RUN chmod +x mvnw && ./mvnw -B -DskipTests package

FROM eclipse-temurin:21-jre-jammy
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /src/target/social-graph-service-0.1.0.jar /app/app.jar
EXPOSE 8081
USER 65534
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
