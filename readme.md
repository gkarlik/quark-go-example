# quark-go-example

Quark-go-example is a use case of [quark-go](https://github.com/gkarlik/quark-go) project.

**Important:** Work in progress! Some components may be changed in the future. Project will evolve with [quark-go](http://github.com/gkarlik/quark-go) project.

## Installation

Quark-go-example uses [govendor](https://github.com/kardianos/govendor) to manage project dependencies. Execute the following command inside installation directory to synchronize dependencies:

`$ govendor sync`

## Running

The best way to run quark-go-example is to use docker containers. Execute the following commands to build and run project:

`$ docker-compose build`

`$ docker-compose up`

## Sample code highlights

Define service:

```
type MyService struct {
    *quark.ServiceBase
}
```

Initialize service:

```
var service = &MyService{
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
```

Register service in service discovery catalog:

```
err := service.Discovery().RegisterService(sd.WithInfo(service.Info()))
if err != nil {
    service.Log().ErrorWithFields(logger.Fields{
        "err": err,
    }, "Cannot register service")

    panic("Cannot register service!")
}
```

Database access:

```
func (ur *UserRepository) FindByLogin(login string) (*User, error) {
	if login == "" {
		return nil, fmt.Errorf("Invalid username or password")
	}

	var user User
	if err := ur.First(&user, User{Login: login}); err != nil {
		return nil, err
	}
	return &user, nil
}
```