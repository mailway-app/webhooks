package main

import (
	"fmt"
	"net"
	"time"
	"strings"
	"net/http"
	"bytes"

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

func callHook(from string, to []string, subject string) error {
    log.Printf("call hook\n")

/*	TODO(frd): to send param with request, not url, actualy bugged. fix or delete
    var jsonStr = []byte(`{"id": "17455ee7-5b62-4d12-98a1-38ba9950abd8", "data": "[]", "name": "aaaa", "initialState": "2"}`)
    url := "http://127.0.0.1:9080/administration/entretiens/API/exportWorkflow"

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
    req.Header.Set("X-Custom-Header", "myvalue")
    req.Header.Set("Content-Type", "application/json")
//*/

	paramUrl := "?from=" + from + "&to=" + strings.Join(to, ",") + "&subject=" + subject
	// TODO(frd): use config hook url
    url := "http://127.0.0.1:9080/test" + paramUrl
    req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte{}))

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

	log.Printf("response Status : %s", resp.Status)
    fmt.Println(resp)

    return nil
}

func mailHandler(origin net.Addr, from string, to []string, in []byte) error {
	fmt.Printf("%s forwarded an email, %s -> %s\n", origin, from, to)

	tabIn := strings.Split(string(in), "\n")
	longSubject := tabIn[3]
	subject := string(longSubject[9:len(longSubject)])
	fmt.Println(subject)

	callHook(from, to, subject)

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
