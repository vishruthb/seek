.PHONY: build test install clean run

build:
	go build -o seek .

test:
	go test ./...

install: build
	bash install.sh

run:
	go run . "$(QUERY)"

clean:
	rm -f seek
