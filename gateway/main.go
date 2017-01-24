package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gkarlik/quark"
	auth "github.com/gkarlik/quark/auth/jwt"
	"github.com/gkarlik/quark/logger"
	"github.com/gkarlik/quark/ratelimiter"
	sd "github.com/gkarlik/quark/service/discovery"
	"github.com/gkarlik/quark/service/discovery/consul"
	"github.com/gorilla/mux"
)

type gateway struct {
	*quark.ServiceBase
}

func createGateway() *gateway {
	name := quark.GetEnvVar("GATEWAY_NAME")
	version := quark.GetEnvVar("GATEWAY_VERSION")
	gp := quark.GetEnvVar("GATEWAY_PORT")
	discovery := quark.GetEnvVar("DISCOVERY")

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
			quark.Discovery(consul.NewServiceDiscovery(discovery))),
	}
}

var srv = createGateway()

func main() {
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

	srv.Dispose()
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
