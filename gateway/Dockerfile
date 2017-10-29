FROM golang

RUN go get github.com/jinzhu/gorm && \
    go get google.golang.org/grpc && \
    go get github.com/dgrijalva/jwt-go && \
    go get github.com/Sirupsen/logrus && \
    go get github.com/prometheus/client_golang/prometheus && \
    go get github.com/hashicorp/consul/api && \
    go get github.com/openzipkin/zipkin-go-opentracing && \
    go get github.com/opentracing/opentracing-go && \
    go get golang.org/x/time/rate && \
    go get github.com/gorilla/mux && \
    go get github.com/jinzhu/gorm/dialects/postgres && \
    go get github.com/gkarlik/quark-go

COPY . /go/src/github.com/gkarlik/quark-go-example/gateway
WORKDIR /go/src/github.com/gkarlik/quark-go-example/gateway

ENV GATEWAY_NAME=Gateway \
    GATEWAY_VERSION=1.0 \
    GATEWAY_PORT=8888 \
    GATEWAY_SECRET=mysecret \
    DISCOVERY=consul:8500 \
    GATEWAY_DB_DIALECT=postgres \
    GATEWAY_DB_CONN_STR="host=database user=postgres dbname=quark_go_example sslmode=disable password=" \
    TRACER=http://zipkin:9411/api/v1/spans

ENTRYPOINT go run main.go
