FROM carlos

RUN go get github.com/gorilla/mux

COPY . /go/gateway
WORKDIR /go/gateway

ENV GATEWAY_NAME Gateway
ENV GATEWAY_VERSION 1.0
ENV GATEWAY_PORT 9999
ENV GATEWAY_SECRET mysecret
ENV DISCOVERY consul:8500

ENTRYPOINT go run main.go
