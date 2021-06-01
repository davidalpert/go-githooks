
.PHONY: test
test:
	go build -o ../mob-test/.git/hooks/prepare-commit-msg ./cmd/prepare-commit-msg/main.go