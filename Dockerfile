FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./

# Download dengan retry
RUN go mod download || \
    (sleep 5 && go mod download) || \
    (sleep 10 && go mod download)

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o siaga-api

FROM alpine:3.20
WORKDIR /app

ENV TZ=Asia/Jakarta

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories \
    && apk add --no-cache wget ca-certificates tzdata
COPY --from=builder /app/siaga-api .
COPY --from=builder /app/migrations ./migrations
EXPOSE 8686

HEALTHCHECK --interval=5s --timeout=3s --retries=5 \
  CMD wget -qO- http://localhost:8686/health || exit 1

CMD ["./siaga-api"]
