//
// Copyright (c) 2019
// Mainflux
//
// SPDX-License-Identifier: Apache-2.0
//

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/mainflux/mainflux"
	"github.com/mainflux/mainflux/logger"
	"github.com/mainflux/mfxkit/mfxkit"
	"github.com/mainflux/mfxkit/mfxkit/api"
	mfxkithttpapi "github.com/mainflux/mfxkit/mfxkit/api/mfxkit/http"

	kitprometheus "github.com/go-kit/kit/metrics/prometheus"
	opentracing "github.com/opentracing/opentracing-go"
	stdprometheus "github.com/prometheus/client_golang/prometheus"
	jconfig "github.com/uber/jaeger-client-go/config"
)

const (
	defLogLevel   = "error"
	defHTTPPort   = "9021"
	defJaegerURL  = ""
	defServerCert = ""
	defServerKey  = ""
	defSecret     = "secret"

	envLogLevel   = "MF_MFXKIT_LOG_LEVEL"
	envHTTPPort   = "MF_MFXKIT_HTTP_PORT"
	envServerCert = "MF_MFXKIT_SERVER_CERT"
	envServerKey  = "MF_MFXKIT_SERVER_KEY"
	envSecret     = "MF_MFXKIT_SECRET"
	envJaegerURL  = "MF_JAEGER_URL"
)

type config struct {
	logLevel     string
	httpPort     string
	authHTTPPort string
	authGRPCPort string
	serverCert   string
	serverKey    string
	secret       string
	jaegerURL    string
}

func main() {
	cfg := loadConfig()

	logger, err := logger.New(os.Stdout, cfg.logLevel)
	if err != nil {
		log.Fatalf(err.Error())
	}

	mfxkitTracer, mfxkitCloser := initJaeger("mfxkit", cfg.jaegerURL, logger)
	defer mfxkitCloser.Close()

	svc := newService(cfg.secret, logger)
	errs := make(chan error, 2)

	go startHTTPServer(mfxkithttpapi.MakeHandler(mfxkitTracer, svc), cfg.httpPort, cfg, logger, errs)

	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT)
		errs <- fmt.Errorf("%s", <-c)
	}()

	err = <-errs
	logger.Error(fmt.Sprintf("Mfxkit service terminated: %s", err))
}

func loadConfig() config {
	return config{
		logLevel:   mainflux.Env(envLogLevel, defLogLevel),
		httpPort:   mainflux.Env(envHTTPPort, defHTTPPort),
		serverCert: mainflux.Env(envServerCert, defServerCert),
		serverKey:  mainflux.Env(envServerKey, defServerKey),
		jaegerURL:  mainflux.Env(envJaegerURL, defJaegerURL),
		secret:     mainflux.Env(envSecret, defSecret),
	}
}

func initJaeger(svcName, url string, logger logger.Logger) (opentracing.Tracer, io.Closer) {
	if url == "" {
		return opentracing.NoopTracer{}, ioutil.NopCloser(nil)
	}

	tracer, closer, err := jconfig.Configuration{
		ServiceName: svcName,
		Sampler: &jconfig.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &jconfig.ReporterConfig{
			LocalAgentHostPort: url,
			LogSpans:           true,
		},
	}.NewTracer()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to init Jaeger client: %s", err))
		os.Exit(1)
	}

	return tracer, closer
}

func newService(secret string, logger logger.Logger) mfxkit.Service {
	svc := mfxkit.New(secret)

	svc = api.LoggingMiddleware(svc, logger)
	svc = api.MetricsMiddleware(
		svc,
		kitprometheus.NewCounterFrom(stdprometheus.CounterOpts{
			Namespace: "mfxkit",
			Subsystem: "api",
			Name:      "request_count",
			Help:      "Number of requests received.",
		}, []string{"method"}),
		kitprometheus.NewSummaryFrom(stdprometheus.SummaryOpts{
			Namespace: "mfxkit",
			Subsystem: "api",
			Name:      "request_latency_microseconds",
			Help:      "Total duration of requests in microseconds.",
		}, []string{"method"}),
	)

	return svc
}

func startHTTPServer(handler http.Handler, port string, cfg config, logger logger.Logger, errs chan error) {
	p := fmt.Sprintf(":%s", port)
	if cfg.serverCert != "" || cfg.serverKey != "" {
		logger.Info(fmt.Sprintf("Mfxkit service started using https on port %s with cert %s key %s",
			port, cfg.serverCert, cfg.serverKey))
		errs <- http.ListenAndServeTLS(p, cfg.serverCert, cfg.serverKey, handler)
		return
	}
	logger.Info(fmt.Sprintf("Mfxkit service started using http on port %s", cfg.httpPort))
	errs <- http.ListenAndServe(p, handler)
}
