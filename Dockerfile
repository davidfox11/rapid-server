FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
RUN go build -o /app/server ./cmd/server

FROM alpine:latest
COPY --from=builder /app/server /app/server
EXPOSE 8080
CMD ["/app/server"]
