FIXTURE_MODULES = IF-MIB SNMPv2-MIB IP-MIB ENTITY-MIB BRIDGE-MIB
FIXTURE_DIR = testdata/fixtures/netsnmp
CORPUS_PATH = testdata/corpus/primary

FUZZ_TARGETS := \
	./internal/lexer/FuzzTokenize \
	./internal/cst/parser/FuzzCSTParseModule \
	./internal/cst/parser/FuzzCSTConstraintParsing \
	./internal/graph/FuzzResolutionOrder \
	./internal/model/FuzzParseOID \
	./internal/resolver/FuzzFormatOID \
	./FuzzParse \
	./FuzzPipeline \
	./FuzzMultiModule
FUZZ_NAMES := $(notdir $(FUZZ_TARGETS))

.PHONY: $(FUZZ_NAMES) fixtures gomib-netsnmp test lint fuzz

gomib-netsnmp:
	CGO_ENABLED=1 go build -tags cgo -o gomib-netsnmp ./cmd/gomib-netsnmp

fixtures: gomib-netsnmp
	@mkdir -p $(FIXTURE_DIR)
	./gomib-netsnmp fixturegen -p $(CORPUS_PATH) -dir $(FIXTURE_DIR) $(FIXTURE_MODULES)

test:
	go test ./...

lint:
	golangci-lint run ./...

fuzz: $(FUZZ_NAMES)

# Go exits successfully when -fuzz matches nothing, even in packages without tests.
$(FUZZ_NAMES): package = $(dir $(filter %/$@,$(FUZZ_TARGETS)))
$(FUZZ_NAMES):
	@go test -json=false -list '^$@$$' $(package) | grep -Fx '$@' >/dev/null || { echo "Missing $@ in $(package)" >&2; exit 1; }
	go test -run '^$$' -fuzz '^$@$$' -fuzztime=$(or $(FUZZ_TIME),30s) $(package)
