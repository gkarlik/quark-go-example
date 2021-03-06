package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gkarlik/quark-go"
	"github.com/gkarlik/quark-go/broker/rabbitmq"
	"github.com/gkarlik/quark-go/logger"
	"github.com/gkarlik/quark-go/metrics"
	"github.com/gkarlik/quark-go/metrics/prometheus"
	sd "github.com/gkarlik/quark-go/service/discovery"
	"github.com/gkarlik/quark-go/service/discovery/consul"
	"github.com/gkarlik/quark-go/service/trace/zipkin"
	"github.com/gorilla/mux"
	"github.com/opentracing/opentracing-go"
)

// multiplyService service based on quark.ServiceBase
type multiplyService struct {
	*quark.ServiceBase
}

var (
	errorCounter metrics.Counter
)

// helper function to initialize multiplyService service
func createMultiplyService() *multiplyService {
	// load settings from environment variables
	name := quark.GetEnvVar("MULTIPLY_SERVICE_NAME")
	version := quark.GetEnvVar("MULTIPLY_SERVICE_VERSION")
	gp := quark.GetEnvVar("MULTIPLY_SERVICE_PORT")
	discovery := quark.GetEnvVar("DISCOVERY")
	tAddr := quark.GetEnvVar("TRACER")
	bAddr := quark.GetEnvVar("BROKER")

	port, err := strconv.Atoi(gp)
	if err != nil {
		panic("Incorrect port value!")
	}

	addr, err := quark.GetHostAddress(port)
	if err != nil {
		panic("Cannot resolve host address!")
	}

	// initialize multiplyService service
	m := &multiplyService{
		ServiceBase: quark.NewService(
			quark.Name(name),
			quark.Version(version),
			quark.Address(addr),
			quark.Discovery(consul.NewServiceDiscovery(discovery)),
			quark.Metrics(prometheus.NewMetricsExposer()),
			quark.Tracer(zipkin.NewTracer(tAddr, name, addr)),
			quark.Broker(rabbitmq.NewMessageBroker(bAddr))),
	}
	m.Log().SetLevel(logger.DebugLevel)

	errorCounter = m.Metrics().CreateCounter("error_count", "Counting errors")

	return m
}

var srv = createMultiplyService()

func main() {
	defer srv.Dispose()

	// register service in service discovery catalog
	err := srv.Discovery().RegisterService(sd.WithInfo(srv.Info()))
	if err != nil {
		srv.Log().ErrorWithFields(logger.Fields{
			"err": err,
		}, "Cannot register service")

		panic("Cannot register service!")
	}

	r := mux.NewRouter()
	r.HandleFunc("/multiply/{a:[0-9]+}/{b:[0-9]+}", mulitplyHandler)
	r.Handle("/metrics", srv.Metrics().ExposeHandler())

	go func() {
		srv.Log().Info("Waiting for incomming messages")

		messages, err := srv.Broker().Subscribe(context.Background(), "SampleTopic")
		if err != nil {
			srv.Log().ErrorWithFields(logger.Fields{
				"error": err,
			}, "Cannot subscribe to messages with topic = 'SampleTopic'")

			return
		}
		for msg := range messages {
			srv.Log().InfoWithFields(logger.Fields{
				"topic": msg.Topic,
				"value": string(msg.Value.([]byte)),
			}, "Message received")
		}
	}()

	srv.Log().InfoWithFields(logger.Fields{
		"addr": srv.Info().Address.Host,
	}, "Service initialized. Listening for incomming connections")

	srv.Log().Fatal(http.ListenAndServe(srv.Info().Address.Host, r))
}

// function to handle multiplication of two integers
func mulitplyHandler(w http.ResponseWriter, r *http.Request) {
	// extract and start request tracing span
	span, _ := srv.Tracer().ExtractSpan("mul_handler", opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(r.Header))
	defer span.Finish()

	// multiply two integers
	srv.Log().Info("Executing multiply function")

	if time.Now().Second()%2 == 0 {
		errorCounter.Inc()

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	vars := mux.Vars(r)

	a, _ := strconv.Atoi(vars["a"])
	b, _ := strconv.Atoi(vars["b"])

	// generate response
	resp := fmt.Sprintf("%d * %d = %d", a, b, a*b)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resp))
}
