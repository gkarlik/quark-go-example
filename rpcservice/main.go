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
	"github.com/gkarlik/quark-go/metrics/influxdb"
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
	mAddr := quark.GetEnvVar("METRICS_ADDRES")
	mDatabase := quark.GetEnvVar("METRICS_DATABASE")
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
	return &sumService{
		ServiceBase: quark.NewService(
			quark.Name(name),
			quark.Version(version),
			quark.Address(addr),
			quark.Discovery(consul.NewServiceDiscovery(discovery)),
			quark.Metrics(influxdb.NewMetricsReporter(mAddr,
				influxdb.Database(mDatabase),
				influxdb.Username(""),
				influxdb.Password(""),
			)),
			quark.Tracer(zipkin.NewTracer(tAddr, name, addr)),
			quark.Broker(rabbitmq.NewMessageBroker(bAddr))),
	}
}

var srv = createSumService()

// function to handle sum of two integers
func (s *sumService) Sum(ctx context.Context, r *proxy.SumRequest) (*proxy.SumResponse, error) {
	// extract and start request tracing span
	span := quark.StartRPCSpan(srv, "sum_handler", ctx)
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
	defer srv.Dispose()

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
				Key:   "SampleTopic",
				Value: "Sample message with timestamp = " + time.Now().String(),
			}

			srv.Log().InfoWithFields(logger.Fields{
				"topic": msg.Key,
				"value": msg.Value,
			}, "Sending message")

			if err := srv.Broker().PublishMessage(msg); err != nil {
				srv.Log().ErrorWithFields(logger.Fields{
					"error": err,
				}, "Cannot publish message")
			}

			// 1 - 5 seconds delay
			delay := time.Duration(r.Int63n(5) + 1)
			time.Sleep(delay * time.Second)
		}
	}()

	// create and start RPC server
	server := gRPC.NewServer()
	server.Start(srv)
}
