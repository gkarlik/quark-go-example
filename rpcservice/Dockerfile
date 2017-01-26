FROM carlos

COPY . /go/src/github.com/gkarlik/quark-example/rpcservice
WORKDIR /go/src/github.com/gkarlik/quark-example/rpcservice

ENV SUM_SERVICE_NAME SumService
ENV SUM_SERVICE_VERSION 1.0
ENV SUM_SERVICE_PORT 6666
ENV DISCOVERY consul:8500

ENV METRICS_ADDRES http://influxdb:8086
ENV METRICS_DATABASE test

ENV TRACER http://zipkin:9411/api/v1/spans

ENTRYPOINT go run main.go
