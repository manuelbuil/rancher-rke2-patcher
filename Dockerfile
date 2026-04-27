# Minimal Dockerfile for rke2-patcher
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /rke2-patcher .

FROM scratch
COPY --from=builder /rke2-patcher /rke2-patcher
ENTRYPOINT ["/rke2-patcher"]
