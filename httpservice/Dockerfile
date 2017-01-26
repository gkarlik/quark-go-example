FROM carlos

RUN go get github.com/gorilla/mux

COPY . /go/src/github.com/gkarlik/quark-example/httpservice
WORKDIR /go/src/github.com/gkarlik/quark-example/httpservice

ENV MULTIPLY_SERVICE_NAME MultiplyService
ENV MULTIPLY_SERVICE_VERSION 1.0
ENV MULTIPLY_SERVICE_PORT 7777
ENV DISCOVERY consul:8500

ENV METRICS_ADDRES http://influxdb:8086
ENV METRICS_DATABASE test

ENV TRACER http://zipkin:9411/api/v1/spans

ENTRYPOINT go run main.go
