package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/mailway-app/config"

	"github.com/mhale/smtpd"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// https://www.iana.org/assignments/smtp-enhanced-status-codes/smtp-enhanced-status-codes.xhtml
var (
	internalError = errors.New("451 4.3.0 Internal server errror")
)

const (
	INT_HEADER_PREFIX = "mw-int-"
)

type WebhookPayload struct {
	Headers mail.Header `json:"headers"`
	BodyURL string      `json:"bodyURL"`
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

func callWebHook(wp WebhookPayload, url string, uuid string, domain string) error {
	jsonData, err := json.Marshal(wp)
	if err != nil {
		return errors.Wrap(err, "could not serialize request payload")
	}

	signature := ""

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mw-Domain", domain)
	req.Header.Set("Mw-Id", uuid)
	req.Header.Set("Mw-Signature", signature)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "could not send request")
	}
	defer resp.Body.Close()

	log.Infof("webhook returned: %d %s", resp.StatusCode, resp.Status)
	return nil
}

func mailHandler(origin net.Addr, from string, to []string, in []byte) error {
	msg, err := mail.ReadMessage(bytes.NewReader(in))
	if err != nil {
		log.Errorf("failed to parse message: %s", err)
		return internalError
	}

	url := msg.Header.Get("Mw-Int-Webhook-Url")
	urlMail := ""
	uuid := msg.Header.Get("Mw-Int-Id")
	domain := msg.Header.Get("Mw-Int-Domain")

	for key := range msg.Header {
		if strings.HasPrefix(strings.ToLower(key), INT_HEADER_PREFIX) {
			delete(msg.Header, key)
		}
	}

	data := &WebhookPayload{
		Headers: msg.Header,
		BodyURL: urlMail,
	}

	if err := callWebHook(*data, url, uuid, domain); err != nil {
		log.Errorf("could not call webhook: %s", err)
		return internalError
	}
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
