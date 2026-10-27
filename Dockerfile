FROM golang:1.24-alpine AS builder

WORKDIR /app

ENV CGO_ENABLED=0
ENV GOOS=linux

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o gateway ./cmd/gateway/main.go

FROM alpine:3.20

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/gateway .

# Create sessions volume mount point
RUN mkdir -p /app/sessions
VOLUME /app/sessions

EXPOSE 8989

CMD ["./gateway"]
