package main

import (
	"fmt"
	"net"
	"time"

	"github.com/mailway-app/config"

	"github.com/mhale/smtpd"
	log "github.com/sirupsen/logrus"
)

func logger(remoteIP, verb, line string) {
	log.Printf("%s %s %s\n", remoteIP, verb, line)
}

func Run(addr string) error {
	smtpd.Debug = true
	srv := &smtpd.Server{
		Addr:     addr,
		Handler:  mailHandler,
		Appname:  "webhook",
		Hostname: config.CurrConfig.InstanceHostname,
		Timeout:  3 * time.Minute,
		LogRead:  logger,
		LogWrite: logger,
	}
	log.Infof("Listening on %s", addr)
	return srv.ListenAndServe()
}

func mailHandler(origin net.Addr, from string, to []string, in []byte) error {
	fmt.Printf("%s forwarded an email, %s -> %s", origin, from, to)
	return nil
}

func main() {
	if err := config.Init(); err != nil {
		log.Fatalf("failed to init config: %s", err)
	}

	// TODO(sven): use config version
	// addr := fmt.Sprintf("127.0.0.1:%d", config.CurrConfig.PortWebhooks)
	addr := fmt.Sprintf("127.0.0.1:%d", 2526)
	if err := Run(addr); err != nil {
		log.Fatal(err)
	}
}
