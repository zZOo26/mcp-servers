ARG SERVICE=serper

FROM golang:1.25-alpine AS builder
ARG SERVICE
WORKDIR /build
COPY shared/ ./shared/
COPY ${SERVICE}/ ./${SERVICE}/
WORKDIR /build/${SERVICE}
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server .

FROM alpine:3.20
RUN apk add --no-cache wget
ARG SERVICE
COPY --from=builder /build/${SERVICE}/server /server
ENTRYPOINT ["/server"]
