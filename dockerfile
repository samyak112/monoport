
# Stage 1: Build
FROM golang:1.23-alpine AS builder

WORKDIR /monoport

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o monoport .

# Stage 2: Run
FROM alpine:latest

WORKDIR /monoport

COPY --from=builder /monoport/monoport .

EXPOSE 8000
EXPOSE 5000/udp

CMD ["./monoport"]
