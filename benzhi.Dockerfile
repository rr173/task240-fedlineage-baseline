# benzhi.Dockerfile 与 Dockerfile 同源，供 build_benzhi_docker.sh 引用。
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
