package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/mailway-app/config"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/mhale/smtpd"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// https://www.iana.org/assignments/smtp-enhanced-status-codes/smtp-enhanced-status-codes.xhtml
var (
	internalError = errors.New("451 4.3.0 Internal server errror")
	httpClient    *retryablehttp.Client
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

func saveBuffer(id string) error {
	src := fmt.Sprintf("/tmp/%s.eml", id)
	dest := fmt.Sprintf("/usr/local/lib/maildb/%s.eml", id)
	err := os.Rename(src, dest)
	if err != nil {
		return errors.Wrap(err, "could not move file")
	}
	return nil
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

func callWebHook(wp WebhookPayload, url string, id string, domain string) error {
	jsonData, err := json.Marshal(wp)
	if err != nil {
		return errors.Wrap(err, "could not serialize request payload")
	}

	signature := ""

	req, err := retryablehttp.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Mw-Domain", domain)
	req.Header.Set("Mw-Id", id)
	req.Header.Set("Mw-Signature", signature)

	resp, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "could not send request")
	}
	defer resp.Body.Close()

	log.Infof("webhook returned: %d %s", resp.StatusCode, resp.Status)

	if resp.StatusCode != 200 {
		return errors.Errorf("webhook returned %d %s", resp.StatusCode, resp.Status)
	} else {
		return nil
	}
}

func mailHandler(origin net.Addr, from string, to []string, in []byte) error {
	msg, err := mail.ReadMessage(bytes.NewReader(in))
	if err != nil {
		log.Errorf("failed to parse message: %s", err)
		return internalError
	}

	id := msg.Header.Get("Mw-Int-Id")

	if err := saveBuffer(id); err != nil {
		log.Errorf("failed to save email buffer: %s", err)
		return internalError
	}

	url := msg.Header.Get("Mw-Int-Webhook-Url")
	urlMail := ""
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

	if err := callWebHook(*data, url, id, domain); err != nil {
		if err := updateMailStatus(config.CurrConfig.ServerJWT, domain, id, MAIL_STATUS_DELIVERY_ERROR); err != nil {
			log.Errorf("could not update email status in maildb: %s", err)
		}
		log.Warnf("could not call webhook: %s", err)
		return internalError
	}
	if err := updateMailStatus(config.CurrConfig.ServerJWT, domain, id, MAIL_STATUS_DELIVERED); err != nil {
		log.Errorf("could not update email status in maildb: %s", err)
	}
	return nil
}

func main() {
	if err := config.Init(); err != nil {
		log.Fatalf("failed to init config: %s", err)
	}

	httpClient = retryablehttp.NewClient()
	httpClient.RetryMax = 5
	httpClient.HTTPClient = &http.Client{
		Timeout: 60 * time.Second,
	}

	addr := fmt.Sprintf("127.0.0.1:%d", config.CurrConfig.PortWebhook)
	if err := Run(addr); err != nil {
		log.Fatal(err)
	}
}
