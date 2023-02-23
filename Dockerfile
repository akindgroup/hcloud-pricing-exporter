ARG ARCH=""
FROM golang:alpine

RUN apk add curl

# Create application directory
RUN mkdir /app
ADD . /app/
WORKDIR /app

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${ARCH} go build -o run .

# Add the execution user
RUN adduser -S -D -H -h /app execuser
USER execuser

# Run the application
ENTRYPOINT ["./run", "-additional-labels", "service,environment,owner"]
