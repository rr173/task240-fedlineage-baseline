# 语法：FROM 必须放首行
FROM golang:1.26.3-bookworm

ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=local
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn
ENV GO111MODULE=on

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /app/fedlineage ./cmd/fedlineage

ENTRYPOINT ["/app/fedlineage"]
CMD ["--smoke-test"]
