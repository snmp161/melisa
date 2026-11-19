GOLANGCI_LINT_VERSION := v2.15.0
GOVULNCHECK_VERSION := v1.10.0

.PHONY: all build clean generate fmt-check tidy-check test test-integration test-validation vet lint vuln check

all: build

build:
	go build -o melisa ./cmd/melisa

# Compile BPF programs into internal/ebpf/bpf/*.o for embedding via //go:embed.
# Requires (on the host running this target):
#   - clang, llvm        (apt: clang llvm)
#   - bpftool            (apt: linux-tools-generic, then ln -s to /usr/local/bin)
#   - libbpf headers     (apt: libbpf-dev)
#   - kernel BTF         (/sys/kernel/btf/vmlinux — kernel built with CONFIG_DEBUG_INFO_BTF=y)
#
# The generated vmlinux.h is host-kernel specific but the resulting .o is CO-RE
# (BPF Type Format-relocatable) and will run on any kernel with BTF support.
# macOS/dev machines without these tools should skip `make generate` — the
# build will then rely on the on-disk fallback in loader.go.
generate:
	@command -v clang >/dev/null || { echo "ERROR: clang not found (install clang/llvm)"; exit 1; }
	@command -v bpftool >/dev/null || { echo "ERROR: bpftool not found (install linux-tools-generic)"; exit 1; }
	@test -f /sys/kernel/btf/vmlinux || { echo "ERROR: /sys/kernel/btf/vmlinux not present (kernel needs CONFIG_DEBUG_INFO_BTF=y)"; exit 1; }
	mkdir -p internal/ebpf/bpf
	bpftool btf dump file /sys/kernel/btf/vmlinux format c > internal/ebpf/c/vmlinux.h
	clang -g -O2 -target bpf -D__TARGET_ARCH_x86 \
		-I/usr/include/x86_64-linux-gnu \
		-I internal/ebpf/c \
		-c internal/ebpf/c/tcpretrans.bpf.c \
		-o internal/ebpf/bpf/tcpretrans.o

fmt-check:
	@unformatted="$$(gofmt -l cmd internal)"; \
	if [ -n "$$unformatted" ]; then \
		echo "Go files need formatting:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

tidy-check:
	go mod tidy -diff

test:
	go test -race -count=1 -timeout 120s ./...

vet:
	go vet ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run --timeout=5m

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

check: fmt-check tidy-check test vet lint vuln
	@echo "All checks passed."

test-integration:
	bash tests/integration/docker_test.sh

test-validation:
	bash tests/validation/run_validation.sh

clean:
	rm -f melisa
	rm -f internal/ebpf/bpf/*.o
	rm -f internal/ebpf/c/vmlinux.h
