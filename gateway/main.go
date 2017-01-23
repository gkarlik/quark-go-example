package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"errors"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gkarlik/quark"
	auth "github.com/gkarlik/quark/auth/jwt"
	"github.com/gkarlik/quark/logger"
	"github.com/gkarlik/quark/logger/logrus"
	"github.com/gkarlik/quark/ratelimiter"
	sd "github.com/gkarlik/quark/service/discovery"
	"github.com/gkarlik/quark/service/discovery/consul"
	"github.com/gorilla/mux"
	"io/ioutil"
)

type gateway struct {
	*quark.ServiceBase
}

func getEnvVar(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("Environment variable %q not set!", key))
	}
	return v
}

func createGateway() *gateway {
	name := getEnvVar("GATEWAY_NAME")
	version := getEnvVar("GATEWAY_VERSION")
	gp := getEnvVar("GATEWAY_PORT")
	discovery := getEnvVar("DISCOVERY")

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
			quark.Logger(logrus.NewLogger()),
			quark.Discovery(consul.NewServiceDiscovery(discovery))),
	}
}

var srv = createGateway()

func main() {
	secret := getEnvVar("GATEWAY_SECRET")
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
	vars := mux.Vars(r)

	a, _ := strconv.Atoi(vars["a"])
	b, _ := strconv.Atoi(vars["b"])

	resp := fmt.Sprintf("%d + %d = %d", a, b, a+b)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resp))
}

func multiplyHandler(w http.ResponseWriter, r *http.Request) {
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
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://%s/multiply/%d/%d", url.String(), a, b), nil)
		client := http.Client{
			Timeout: 10 * time.Second,
		}

		resp, _ := client.Do(req)
		data, _ := ioutil.ReadAll(resp.Body)

		w.WriteHeader(http.StatusOK)
		w.Write(data)

		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}
