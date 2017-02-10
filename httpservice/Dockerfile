FROM golang

RUN go get google.golang.org/grpc && \
    go get github.com/dgrijalva/jwt-go && \
    go get github.com/Sirupsen/logrus && \
    go get github.com/influxdata/influxdb/client/v2 && \
    go get github.com/hashicorp/consul/api && \
    go get github.com/openzipkin/zipkin-go-opentracing && \
    go get github.com/opentracing/opentracing-go && \
    go get github.com/gorilla/mux && \
    go get github.com/streadway/amqp && \
    go get github.com/gkarlik/quark-go

COPY . /go/src/github.com/gkarlik/quark-go-example/httpservice
WORKDIR /go/src/github.com/gkarlik/quark-go-example/httpservice

ENV MULTIPLY_SERVICE_NAME=MultiplyService \
    MULTIPLY_SERVICE_VERSION=1.0 \
    MULTIPLY_SERVICE_PORT=7777 \
    DISCOVERY=consul:8500 \
    METRICS_ADDRES=http://influxdb:8086 \
    METRICS_DATABASE=test \
    TRACER=http://zipkin:9411/api/v1/spans \
    BROKER=amqp://rabbitmq:5672/

ENTRYPOINT go run main.go
