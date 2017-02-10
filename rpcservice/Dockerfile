FROM golang

RUN go get google.golang.org/grpc && \
    go get github.com/dgrijalva/jwt-go && \
    go get github.com/Sirupsen/logrus && \
    go get github.com/influxdata/influxdb/client/v2 && \
    go get github.com/hashicorp/consul/api && \
    go get github.com/openzipkin/zipkin-go-opentracing && \
    go get github.com/opentracing/opentracing-go && \
    go get github.com/streadway/amqp && \
    go get github.com/gkarlik/quark-go

COPY . /go/src/github.com/gkarlik/quark-go-example/rpcservice
WORKDIR /go/src/github.com/gkarlik/quark-go-example/rpcservice

ENV SUM_SERVICE_NAME=SumService \
    SUM_SERVICE_VERSION=1.0 \
    SUM_SERVICE_PORT=6666 \
    DISCOVERY=consul:8500 \
    METRICS_ADDRES=http://influxdb:8086 \
    METRICS_DATABASE=test \
    TRACER=http://zipkin:9411/api/v1/spans \
    BROKER=amqp://rabbitmq:5672/

ENTRYPOINT go run main.go
