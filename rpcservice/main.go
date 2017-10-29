package main

import (
	"math/rand"
	"strconv"
	"time"

	"github.com/gkarlik/quark-go"
	proxy "github.com/gkarlik/quark-go-example/rpcservice/proxies/sum"
	"github.com/gkarlik/quark-go/broker"
	"github.com/gkarlik/quark-go/broker/rabbitmq"
	"github.com/gkarlik/quark-go/logger"
	"github.com/gkarlik/quark-go/metrics/prometheus"
	sd "github.com/gkarlik/quark-go/service/discovery"
	"github.com/gkarlik/quark-go/service/discovery/consul"
	gRPC "github.com/gkarlik/quark-go/service/rpc/grpc"
	"github.com/gkarlik/quark-go/service/trace/zipkin"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

// sumService service based on quark.ServiceBase
type sumService struct {
	*quark.ServiceBase
}

// helper function to initialize sumService service
func createSumService() *sumService {
	// load settings from environment variables
	name := quark.GetEnvVar("SUM_SERVICE_NAME")
	version := quark.GetEnvVar("SUM_SERVICE_VERSION")
	gp := quark.GetEnvVar("SUM_SERVICE_PORT")
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

	// initialize sumService service
	s := &sumService{
		ServiceBase: quark.NewService(
			quark.Name(name),
			quark.Version(version),
			quark.Address(addr),
			quark.Discovery(consul.NewServiceDiscovery(discovery)),
			quark.Metrics(prometheus.NewMetricsExposer()),
			quark.Tracer(zipkin.NewTracer(tAddr, name, addr)),
			quark.Broker(rabbitmq.NewMessageBroker(bAddr))),
	}
	s.Log().SetLevel(logger.DebugLevel)

	return s
}

var srv = createSumService()

// function to handle sum of two integers
func (s *sumService) Sum(ctx context.Context, r *proxy.SumRequest) (*proxy.SumResponse, error) {
	// extract and start request tracing span
	span := quark.StartRPCSpan(ctx, srv, "sum_handler")
	defer span.Finish()

	// sum two integers
	srv.Log().Info("Executing sum function")

	return &proxy.SumResponse{
		Sum: r.A + r.B,
	}, nil
}

// function to register service in gRPC server
func (s *sumService) RegisterServiceInstance(server interface{}, serviceInstance interface{}) error {
	proxy.RegisterSumServiceServer(server.(*grpc.Server), serviceInstance.(proxy.SumServiceServer))

	return nil
}

func main() {
	// register service in service discovery catalog
	err := srv.Discovery().RegisterService(sd.WithInfo(srv.Info()))
	if err != nil {
		srv.Log().ErrorWithFields(logger.Fields{
			"err": err,
		}, "Cannot register service")

		panic("Cannot register service!")
	}

	go func() {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))

		for {
			msg := broker.Message{
				Topic: "SampleTopic",
				Value: "Sample message with timestamp = " + time.Now().String(),
			}

			srv.Log().InfoWithFields(logger.Fields{
				"topic": msg.Topic,
				"value": msg.Value,
			}, "Sending message")

			if err := srv.Broker().PublishMessage(context.Background(), msg); err != nil {
				srv.Log().ErrorWithFields(logger.Fields{
					"error": err,
				}, "Cannot publish message")
			}

			// 1 - 5 seconds delay
			delay := time.Duration(r.Int63n(5) + 1)
			time.Sleep(delay * time.Second)
		}
	}()

	done := quark.HandleInterrupt(srv)
	server := gRPC.NewServer()
	defer func() {
		server.Dispose()
		srv.Dispose()
	}()

	go func() {
		srv.Metrics().Expose()
	}()

	go func() {
		server.Start(srv)
	}()

	<-done
}
