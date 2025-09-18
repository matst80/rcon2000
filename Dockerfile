# build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o /rcon2000

# final stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /rcon2000 .
COPY public ./public
EXPOSE 1337
CMD ["./rcon2000"]
