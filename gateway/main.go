package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gkarlik/quark-go"
	proxy "github.com/gkarlik/quark-go-example/gateway/proxies/sum"
	auth "github.com/gkarlik/quark-go/auth/jwt"
	"github.com/gkarlik/quark-go/logger"
	"github.com/gkarlik/quark-go/metrics/influxdb"
	"github.com/gkarlik/quark-go/ratelimiter"
	sd "github.com/gkarlik/quark-go/service/discovery"
	"github.com/gkarlik/quark-go/service/discovery/consul"
	"github.com/gkarlik/quark-go/service/trace/zipkin"
	"github.com/gorilla/mux"
	opentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// gateway service based on quark.ServiceBase
type gateway struct {
	*quark.ServiceBase
}

// helper function to initialize gateway service
func createGateway() *gateway {
	// load settings from environment variables
	name := quark.GetEnvVar("GATEWAY_NAME")
	version := quark.GetEnvVar("GATEWAY_VERSION")
	gp := quark.GetEnvVar("GATEWAY_PORT")
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

	// initialize gateway service
	return &gateway{
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

var srv = createGateway()

func main() {
	defer srv.Dispose()

	// setup authentication middleware
	secret := quark.GetEnvVar("GATEWAY_SECRET")
	am := auth.NewAuthenticationMiddleware(
		auth.WithSecret(secret),
		auth.WithContextKey("USER_KEY"),
		auth.WithAuthenticationFunc(func(credentials auth.Credentials) (auth.Claims, error) {
			if credentials.Username == "test" && credentials.Password == "test" {
				return auth.Claims{
					Username: "test",
					StandardClaims: jwt.StandardClaims{
						Issuer:    srv.Info().Address.String(),
						ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
					},
				}, nil
			}
			return auth.Claims{}, errors.New("Invalid username or password")
		}))

	// setup rate limiter
	rl := ratelimiter.NewHTTPRateLimiter(1 * time.Second)

	r := mux.NewRouter()
	// HTTP handler for generating tokens
	r.HandleFunc("/login", am.GenerateToken)

	// setup routes to limit traffic and require authentication
	r.Handle("/api/sum/{a:[0-9]+}/{b:[0-9]+}", rl.Handle(am.Authenticate(http.HandlerFunc(sumHandler))))
	r.Handle("/api/mul/{a:[0-9]+}/{b:[0-9]+}", rl.Handle(am.Authenticate(http.HandlerFunc(multiplyHandler))))

	srv.Log().InfoWithFields(logger.LogFields{
		"addr": srv.Info().Address.String(),
	}, "Service initialized. Listening for incomming connections")

	http.ListenAndServe(srv.Info().Address.String(), r)
}

// function to handle call to RPC service to sum two integers
func sumHandler(w http.ResponseWriter, r *http.Request) {
	// report response time for monitoring purposes
	start := time.Now()
	defer func() {
		quark.ReportServiceValue(srv, "response_time", time.Since(start).Nanoseconds())
	}()

	// handle request tracing span
	span := srv.Tracer().StartSpan("sum_get_request")
	defer span.Finish()

	vars := mux.Vars(r)
	a, _ := strconv.Atoi(vars["a"])
	b, _ := strconv.Atoi(vars["b"])

	// get the address of SumService from service discovery catalog
	url, err := srv.Discovery().GetServiceAddress(sd.ByName("SumService"))
	if err != nil {
		srv.Log().Error(err)

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// connect to RPC service
	conn, err := grpc.Dial(url.String(), grpc.WithInsecure())
	if err != nil {
		srv.Log().Error(err)

		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	client := proxy.NewSumServiceClient(conn)

	// pass request tracing span to RPC service
	md := metadata.Pairs()
	err = srv.Tracer().InjectSpan(span, opentracing.TextMap, quark.RPCMetadataCarrier{MD: &md})
	ctx := metadata.NewContext(context.Background(), md)

	// call RPC service
	result, err := client.Sum(ctx, &proxy.SumRequest{A: int64(a), B: int64(b)})

	if err != nil {
		srv.Log().Error(err)

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// generate response
	resp := fmt.Sprintf("%d + %d = %d", a, b, result.Sum)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resp))
}

// function to handle call to HTTP service to multiply two integers
func multiplyHandler(w http.ResponseWriter, r *http.Request) {
	// report response time for monitoring purposes
	start := time.Now()
	defer func() {
		quark.ReportServiceValue(srv, "response_time", time.Since(start).Nanoseconds())
	}()

	// handle request tracing span
	span := srv.Tracer().StartSpan("mul_get_request")
	defer span.Finish()

	vars := mux.Vars(r)
	a, _ := strconv.Atoi(vars["a"])
	b, _ := strconv.Atoi(vars["b"])

	// get the address of SumService from service discovery catalog
	url, err := srv.Discovery().GetServiceAddress(sd.ByName("MultiplyService"))
	if err != nil {
		srv.Log().Error(err)

		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if url != nil {
		// call HTTP service and pass request tracing span to it
		data, _ := quark.CallHTTPService(srv, http.MethodGet, fmt.Sprintf("http://%s/multiply/%d/%d", url.String(), a, b), nil, span)

		w.WriteHeader(http.StatusOK)
		w.Write(data)

		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}
