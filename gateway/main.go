package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gkarlik/quark"
	proxy "github.com/gkarlik/quark-example/gateway/proxies/sum"
	auth "github.com/gkarlik/quark/auth/jwt"
	"github.com/gkarlik/quark/logger"
	"github.com/gkarlik/quark/metrics/influxdb"
	"github.com/gkarlik/quark/ratelimiter"
	sd "github.com/gkarlik/quark/service/discovery"
	"github.com/gkarlik/quark/service/discovery/consul"
	gRPC "github.com/gkarlik/quark/service/rpc/grpc"
	"github.com/gkarlik/quark/service/trace/zipkin"
	"github.com/gorilla/mux"
	opentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type gateway struct {
	*quark.ServiceBase
}

func createGateway() *gateway {
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

	rl := ratelimiter.NewHTTPRateLimiter(5 * time.Second)

	r := mux.NewRouter()
	r.HandleFunc("/login", am.GenerateToken)
	r.Handle("/api/sum/{a:[0-9]+}/{b:[0-9]+}", rl.Handle(am.Authenticate(http.HandlerFunc(sumHandler))))
	r.Handle("/api/mul/{a:[0-9]+}/{b:[0-9]+}", rl.Handle(am.Authenticate(http.HandlerFunc(multiplyHandler))))

	srv.Log().InfoWithFields(logger.LogFields{
		"addr": srv.Info().Address.String(),
	}, "Service initialized. Listening for incomming connections")

	http.ListenAndServe(srv.Info().Address.String(), r)
}

func sumHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		quark.ReportServiceValue(srv, "response_time", time.Since(start).Nanoseconds())
	}()

	span := srv.Tracer().StartSpan("sum_get_request")
	defer span.Finish()

	vars := mux.Vars(r)
	a, _ := strconv.Atoi(vars["a"])
	b, _ := strconv.Atoi(vars["b"])

	url, err := srv.Discovery().GetServiceAddress(sd.ByName("SumService"))
	if err != nil {
		srv.Log().Error(err)

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	conn, err := grpc.Dial(url.String(), grpc.WithInsecure())
	if err != nil {
		srv.Log().Error(err)

		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	client := proxy.NewSumServiceClient(conn)

	md := metadata.Pairs()
	err = srv.Tracer().InjectSpan(span, opentracing.TextMap, gRPC.MetadataReaderWriter{MD: &md})
	ctx := metadata.NewContext(context.Background(), md)

	srv.Log().InfoWithFields(logger.LogFields{
		"ctx": ctx,
	}, "Context")

	result, err := client.Sum(ctx, &proxy.SumRequest{A: int64(a), B: int64(b)})

	if err != nil {
		srv.Log().Error(err)

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := fmt.Sprintf("%d + %d = %d", a, b, result.Sum)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resp))
}

func multiplyHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		quark.ReportServiceValue(srv, "response_time", time.Since(start).Nanoseconds())
	}()

	span := srv.Tracer().StartSpan("mul_get_request")
	defer span.Finish()

	vars := mux.Vars(r)
	a, _ := strconv.Atoi(vars["a"])
	b, _ := strconv.Atoi(vars["b"])

	url, err := srv.Discovery().GetServiceAddress(sd.ByName("MultiplyService"))
	if err != nil {
		srv.Log().Error(err)

		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if url != nil {
		data, _ := quark.CallHTTPService(srv, http.MethodGet, fmt.Sprintf("http://%s/multiply/%d/%d", url.String(), a, b), nil, span)

		w.WriteHeader(http.StatusOK)
		w.Write(data)

		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}
