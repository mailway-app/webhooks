VERSION = 1.0.0
DIST = $(PWD)/dist
FPM_ARGS =

.PHONY: test
test:
	go test -v ./...

.PHONY: clean
clean:
	rm -rf $(DIST) *.deb

$(DIST)/webhooks: server.go
	mkdir -p $(DIST)
	go build -o $(DIST)/usr/local/sbin/webhooks

.PHONY: deb
deb: $(DIST)/webhooks
	fpm -n webhooks -s dir -t deb --chdir=$(DIST) --version=$(VERSION) $(FPM_ARGS)

