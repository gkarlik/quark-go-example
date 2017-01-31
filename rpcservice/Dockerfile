FROM golang

RUN go get google.golang.org/grpc
RUN go get github.com/dgrijalva/jwt-go
RUN go get github.com/Sirupsen/logrus
RUN go get github.com/influxdata/influxdb/client/v2
RUN go get github.com/hashicorp/consul/api
RUN go get github.com/openzipkin/zipkin-go-opentracing
RUN go get github.com/opentracing/opentracing-go
RUN go get github.com/gkarlik/quark-go

COPY . /go/src/github.com/gkarlik/quark-go-example/rpcservice
WORKDIR /go/src/github.com/gkarlik/quark-go-example/rpcservice

ENV SUM_SERVICE_NAME SumService
ENV SUM_SERVICE_VERSION 1.0
ENV SUM_SERVICE_PORT 6666
ENV DISCOVERY consul:8500

ENV METRICS_ADDRES http://influxdb:8086
ENV METRICS_DATABASE test

ENV TRACER http://zipkin:9411/api/v1/spans

ENTRYPOINT go run main.go
