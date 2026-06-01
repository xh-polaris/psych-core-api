FROM golang:1.25-alpine AS builder

LABEL stage=gobuilder

ENV CGO_ENABLE=1 \
    GOPROXY=https://goproxy.cn,direct

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories

RUN apk update --no-cache && apk add --no-cache tzdata gcc musl-dev g++ make cmake

WORKDIR /build

ADD go.mod .
ADD go.sum .
RUN go mod download

# 重要：创建目录并复制字典文件
RUN mkdir -p /build/dict
# 复制gojieba的字典文件到构建目录
RUN JIEBA_VER=$(grep 'github.com/yanyiwu/gojieba' go.mod | awk '{print $2}') && cp -r /go/pkg/mod/github.com/yanyiwu/gojieba@${JIEBA_VER}/deps/cppjieba/dict/* /build/dict/ || true

COPY . .
RUN sh ./build.sh

FROM alpine
RUN apk add --no-cache libstdc++

COPY --from=builder /usr/share/zoneinfo/Asia/Shanghai /usr/share/zoneinfo/Asia/Shanghai

ENV TZ Asia/Shanghai

WORKDIR /app
COPY --from=builder /build/output /app
COPY --from=builder /build/dict /app/dict

ENV JIEBA_DICT_PATH=/app/dict

CMD ["sh", "./bootstrap.sh"]
