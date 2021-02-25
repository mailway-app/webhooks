package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"time"
	"io/ioutil"

	"github.com/mailway-app/config"

	"github.com/mhale/smtpd"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type WebhookPayload struct  {
    Headers string `json:"headers"`
    BodyURL string `json:"bodyURL"`
}

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

func callHook(wp WebhookPayload, urlWebhook string, uuid string, domain string, signature string) error {
    log.Printf("call hook\n")

    jsonData, err := json.Marshal(wp)
    if err != nil {
        log.Error(err)
        return err
    }

    req, err := http.NewRequest("POST", urlWebhook, bytes.NewBuffer(jsonData))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Mw-Domain", domain)
    req.Header.Set("Mw-Id", uuid)
    req.Header.Set("Mw-Signature", signature)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        log.Error(err)
        return err
    }
    defer resp.Body.Close()

    log.Printf("response Status : %s", resp.Status)
    fmt.Println(resp)

    bodyBytes, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        log.Fatal(err)
    }
    bodyString := string(bodyBytes)
    log.Info(bodyString)

    return nil
}

func mailHandler(origin net.Addr, from string, to []string, in []byte) error {
	msg, err := mail.ReadMessage(bytes.NewReader(in))
	if err != nil {
		return errors.Wrap(err, "could not parse message")
	}

	fmt.Printf("%s forwarded an email, %s -> %s\n", origin, from, to)

    headers, err := json.Marshal(msg.Header)
    if err != nil {
        log.Error(err)
        return err
    }

    urlWebhook := msg.Header.Get("Mw-Int-Webhook-Url")
    urlMail := ""
    uuid := msg.Header.Get("Mw-Int-Id")
    domain := msg.Header.Get("Mw-Int-Domain")

    data := &WebhookPayload{
        Headers: (string)(headers),
        BodyURL: urlMail,
    }

	callHook(*data, urlWebhook, uuid, domain, "signature")

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
