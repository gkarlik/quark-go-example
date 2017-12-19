package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gkarlik/quark-go"
	"github.com/gkarlik/quark-go-example/gateway/model"
	proxy "github.com/gkarlik/quark-go-example/gateway/proxies/sum"
	"github.com/gkarlik/quark-go/data/access/rdbms"
	"github.com/gkarlik/quark-go/data/access/rdbms/gorm"
	"github.com/gkarlik/quark-go/logger"
	"github.com/gkarlik/quark-go/metrics"
	"github.com/gkarlik/quark-go/metrics/prometheus"
	auth "github.com/gkarlik/quark-go/middleware/auth/jwt"
	"github.com/gkarlik/quark-go/middleware/ratelimiter"
	sd "github.com/gkarlik/quark-go/service/discovery"
	"github.com/gkarlik/quark-go/service/discovery/consul"
	"github.com/gkarlik/quark-go/service/trace/zipkin"
	"github.com/gorilla/mux"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	opentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// gateway service based on quark.ServiceBase
type gateway struct {
	*quark.ServiceBase
}

var (
	responseTimeGauge metrics.Gauge
)

// helper function to initialize gateway service
func createGateway() *gateway {
	// load settings from environment variables
	name := quark.GetEnvVar("GATEWAY_NAME")
	version := quark.GetEnvVar("GATEWAY_VERSION")
	gp := quark.GetEnvVar("GATEWAY_PORT")
	discovery := quark.GetEnvVar("DISCOVERY")
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
	g := &gateway{
		ServiceBase: quark.NewService(
			quark.Name(name),
			quark.Version(version),
			quark.Address(addr),
			quark.Discovery(consul.NewServiceDiscovery(discovery)),
			quark.Metrics(prometheus.NewMetricsExposer()),
			quark.Tracer(zipkin.NewTracer(tAddr, name, addr))),
	}
	g.Log().SetLevel(logger.DebugLevel)

	responseTimeGauge = g.Metrics().CreateGauge("response_time", "Request response time")

	return g
}

func NewDbContext() rdbms.DbContext {
	dialect := quark.GetEnvVar("GATEWAY_DB_DIALECT")
	dbConnStr := quark.GetEnvVar("GATEWAY_DB_CONN_STR")

	context, err := gorm.NewDbContext(dialect, dbConnStr)
	if err != nil {
		srv.Log().ErrorWithFields(logger.Fields{"error": err})
		return nil
	}

	context.DB.SingularTable(true)

	return context
}

func InitializeDatabase() {
	context := NewDbContext()
	if context != nil {
		defer context.Dispose()

		context.(*gorm.DbContext).DB.AutoMigrate(&model.User{})

		user := &model.User{
			Login:    "test",
			Password: "test",
		}

		repo := model.NewUserRepository(context)
		repo.Save(user)
	}
}

func authenticateUser(credentials auth.Credentials) (auth.Claims, error) {
	context := NewDbContext()
	if context == nil {
		return auth.Claims{}, errors.New("Invalid username or password")
	}
	defer context.Dispose()

	repo := model.NewUserRepository(context)
	user, err := repo.FindByLogin(credentials.Username)
	if err != nil {
		return auth.Claims{}, errors.New("Invalid username or password")
	}

	// this is simplication - password should be hashed and salted!
	if user.Password == credentials.Password {
		return auth.Claims{
			Username: credentials.Username,
			StandardClaims: jwt.StandardClaims{
				Issuer:    srv.Info().Address.String(),
				ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
			},
		}, nil
	}
	return auth.Claims{}, errors.New("Invalid username or password")
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
			return authenticateUser(credentials)
		}))

	srv.Log().Info("Initializing database schema and data")
	InitializeDatabase()

	// setup rate limiter middleware
	rl := ratelimiter.NewRateLimiterMiddleware(1 * time.Second)

	r := mux.NewRouter()
	// HTTP handler for generating tokens
	r.HandleFunc("/login", am.GenerateToken)

	// setup routes to limit traffic and require authentication
	r.Handle("/api/sum/{a:[0-9]+}/{b:[0-9]+}", rl.Handle(am.Authenticate(http.HandlerFunc(sumHandler))))
	r.Handle("/api/mul/{a:[0-9]+}/{b:[0-9]+}", rl.Handle(am.Authenticate(http.HandlerFunc(multiplyHandler))))
	r.Handle("/metrics", srv.Metrics().ExposeHandler())

	srv.Log().InfoWithFields(logger.Fields{
		"addr": srv.Info().Address.Host,
	}, "Service initialized. Listening for incomming connections")

	srv.Log().Fatal(http.ListenAndServe(srv.Info().Address.Host, r))
}

// function to handle call to RPC service to sum two integers
func sumHandler(w http.ResponseWriter, r *http.Request) {
	// report response time for monitoring purposes
	start := time.Now()
	defer func() {
		responseTimeGauge.Set(float64(time.Since(start).Nanoseconds()))
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

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// connect to RPC service
	conn, err := grpc.Dial(url.Host, grpc.WithInsecure())
	if err != nil {
		srv.Log().Error(err)

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	client := proxy.NewSumServiceClient(conn)

	// pass request tracing span to RPC service
	md := metadata.Pairs()
	err = srv.Tracer().InjectSpan(span, opentracing.TextMap, quark.RPCMetadataCarrier{MD: &md})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	// call RPC service
	result, err := client.Sum(ctx, &proxy.SumRequest{A: int64(a), B: int64(b)})

	if err != nil {
		srv.Log().Error(err)

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
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
		responseTimeGauge.Set(float64(time.Since(start).Nanoseconds()))
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

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if url != nil {
		// call HTTP service and pass request tracing span to it
		data, err := quark.CallHTTPService(srv, http.MethodGet, fmt.Sprintf("http://%s/multiply/%d/%d", url.Host, a, b), nil, span)
		if err != nil {
			srv.Log().Error(err)

			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(data)

		return
	}
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
