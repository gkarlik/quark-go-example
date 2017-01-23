package main

import (
	"fmt"
	"github.com/gkarlik/quark"
	"github.com/gkarlik/quark/logger"
	"github.com/gkarlik/quark/logger/logrus"
	sd "github.com/gkarlik/quark/service/discovery"
	"github.com/gkarlik/quark/service/discovery/consul"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"strconv"
)

type multiplyService struct {
	*quark.ServiceBase
}

func getEnvVar(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("Environment variable %q not set!", key))
	}
	return v
}

func createMultiplyService() *multiplyService {
	name := getEnvVar("MULTIPLY_SERVICE_NAME")
	version := getEnvVar("MULTIPLY_SERVICE_VERSION")
	gp := getEnvVar("MULTIPLY_SERVICE_PORT")
	discovery := getEnvVar("DISCOVERY")

	port, err := strconv.Atoi(gp)
	if err != nil {
		panic("Incorrect port value!")
	}

	addr, err := quark.GetHostAddress(port)
	if err != nil {
		panic("Cannot resolve host address!")
	}

	return &multiplyService{
		ServiceBase: quark.NewService(
			quark.Name(name),
			quark.Version(version),
			quark.Address(addr),
			quark.Logger(logrus.NewLogger()),
			quark.Discovery(consul.NewServiceDiscovery(discovery))),
	}
}

var srv = createMultiplyService()

func main() {
	err := srv.Discovery().RegisterService(sd.WithInfo(srv.Info()))
	if err != nil {
		panic("Cannot register service!")
	}

	r := mux.NewRouter()
	r.HandleFunc("/multiply/{a:[0-9]+}/{b:[0-9]+}", mulitplyHandler)

	srv.Log().InfoWithFields(logger.LogFields{
		"addr": srv.Info().Address.String(),
	}, "Service initialized. Listening for incomming connections")

	http.ListenAndServe(srv.Info().Address.String(), r)
}

func mulitplyHandler(w http.ResponseWriter, r *http.Request) {
	srv.Log().Info("Handling multiply request")

	vars := mux.Vars(r)

	a, _ := strconv.Atoi(vars["a"])
	b, _ := strconv.Atoi(vars["b"])

	resp := fmt.Sprintf("%d * %d = %d", a, b, a*b)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(resp))
}
