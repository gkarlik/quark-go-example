FROM carlos

RUN go get github.com/gorilla/mux

COPY . /go/src/github.com/gkarlik/quark-example/gateway
WORKDIR /go/src/github.com/gkarlik/quark-example/gateway

ENV GATEWAY_NAME Gateway
ENV GATEWAY_VERSION 1.0
ENV GATEWAY_PORT 9999
ENV GATEWAY_SECRET mysecret
ENV DISCOVERY consul:8500

ENV METRICS_ADDRES http://influxdb:8086
ENV METRICS_DATABASE test

ENV TRACER http://zipkin:9411/api/v1/spans

ENTRYPOINT go run main.go
