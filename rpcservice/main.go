package main

import (
	"strconv"

	"github.com/gkarlik/quark"
	proxy "github.com/gkarlik/quark-example/rpcservice/proxies/sum"
	"github.com/gkarlik/quark/logger"
	"github.com/gkarlik/quark/metrics/influxdb"
	sd "github.com/gkarlik/quark/service/discovery"
	"github.com/gkarlik/quark/service/discovery/consul"
	gRPC "github.com/gkarlik/quark/service/rpc/grpc"
	"github.com/gkarlik/quark/service/trace"
	"github.com/gkarlik/quark/service/trace/zipkin"
	opentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type sumService struct {
	*quark.ServiceBase
}

func createSumService() *sumService {
	name := quark.GetEnvVar("SUM_SERVICE_NAME")
	version := quark.GetEnvVar("SUM_SERVICE_VERSION")
	gp := quark.GetEnvVar("SUM_SERVICE_PORT")
	discovery := quark.GetEnvVar("DISCOVERY")
	mAddr := quark.GetEnvVar("METRICS_ADDRES")
	mDatabase := quark.GetEnvVar("METRICS_DATABASE")
	tAddr := quark.GetEnvVar("TRACER")

	port, err := strconv.Atoi(gp)
	if err != nil {
		panic("Incorrect port value!")
	}

	addr, err := quark.GetHostAddress(port)
	if err != nil {
		panic("Cannot resolve host address!")
	}

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
			quark.Tracer(zipkin.NewTracer(tAddr, name, addr))),
	}
}

var srv = createSumService()

func (s *sumService) Sum(ctx context.Context, r *proxy.SumRequest) (*proxy.SumResponse, error) {
	var span trace.Span

	md, ok := metadata.FromContext(ctx)
	if ok {
		span, _ = srv.Tracer().ExtractSpan("sum_rpc_method", opentracing.TextMap, gRPC.MetadataReaderWriter{MD: &md})
	} else {
		span = srv.Tracer().StartSpan("sum_rpc_method")
	}

	defer span.Finish()

	srv.Log().Info("Executing sum function")

	return &proxy.SumResponse{
		Sum: r.A + r.B,
	}, nil
}

func (s *sumService) RegisterServiceInstance(server interface{}, serviceInstance interface{}) error {
	proxy.RegisterSumServiceServer(server.(*grpc.Server), serviceInstance.(proxy.SumServiceServer))

	return nil
}

func main() {
	defer srv.Dispose()

	err := srv.Discovery().RegisterService(sd.WithInfo(srv.Info()))
	if err != nil {
		srv.Log().ErrorWithFields(logger.LogFields{
			"err": err,
		}, "Cannot register service")

		panic("Cannot register service!")
	}

	server := gRPC.NewServer()
	server.Start(srv)
}
