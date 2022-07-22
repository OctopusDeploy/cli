EXE=
ifeq ($(GOOS),windows)
EXE = .exe
endif

RMCMD = rm -rf
ifeq ($(GOOS),windows)
rmdir /s /q bin/
endif

ifeq (run,$(firstword $(MAKECMDGOALS)))
  RUN_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  $(eval $(RUN_ARGS):;@:)
endif

.PHONY: bin/octopus$(EXE)
bin/octopus$(EXE):
	go build -o bin/octopus cmd/octopus/main.go

.PHONY: run
run:
	go run cmd/octopus/main.go $(RUN_ARGS)

.PHONY: clean
clean:
	$(RMCMD) bin/

## Install/Uninstall (*nix Only)

DESTDIR :=
prefix  := /usr/local
bindir  := ${prefix}/bin

.PHONY: install
install: bin/octopus
	install -d ${DESTDIR}${bindir}
	install -m755 bin/octopus ${DESTDIR}${bindir}/

.PHONY: uninstall
uninstall:
	rm -f ${DESTDIR}${bindir}/octopus
