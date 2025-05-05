# Specifies a parent image
FROM golang:1.24.2-bookworm

ENV RPC_ENDPOINT $RPC_ENDPOINT
ENV DB_CONN $DB_CONNECTION_STRING
# Creates an app directory to hold your app’s source code
WORKDIR /app
 
# Copies everything from your root directory into /app
COPY . .
 
# Installs Go dependencies
RUN go mod download
 
# Builds your app with optional configuration
RUN go build -o /mev-rpc-tracker cmd/httpserver/main.go 
 
# Tells Docker which network port your container listens on
EXPOSE 8080
EXPOSE 8090
 
# Specifies the executable command that runs when the container starts
CMD [ "sh", "-c", "/mev-rpc-tracker --rpc-endpoint $RPC_ENDPOINT --db-connection-string $DB_CONNECTION_STRING"]
