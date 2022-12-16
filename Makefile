RELEASE?=0.0.1
COMMIT?=$(shell git rev-parse --short HEAD)
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
PROJECT?=github.com/undeadops/webby/pkg
APP?=webby
PORT?=5000

clean:
	rm -f ${APP}

run: build
	PORT=${PORT} ./${APP}

test:
	go test -v -race ./...

build: clean
	go build \
		-ldflags "-s -w -X ${PROJECT}/version.Release=${RELEASE} \
		-X ${PROJECT}/version.Commit=${COMMIT} -X ${PROJECT}/version.BuildTime=${BUILD_TIME}" \
		-o ${APP}

docker: build
	docker build -t ghcr.io/undeadops/webby:${RELEASE} . --build-arg RELEASE=${RELEASE} --build-arg COMMIT=${COMMIT} --build-arg BUILD_TIME=${BUILD_TIME}
