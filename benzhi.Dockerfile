# 纯 Go 多阶段镜像：builder 固定 Go 版本，runtime 只保留无 CGO 二进制。
FROM docker.m.daocloud.io/library/golang:1.26.3-bookworm AS builder

ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/fedlineage ./cmd/fedlineage

FROM docker.m.daocloud.io/library/alpine:3.20
WORKDIR /app
COPY --from=builder /out/fedlineage /app/fedlineage
ENTRYPOINT ["/app/fedlineage"]
CMD ["--smoke-test"]
