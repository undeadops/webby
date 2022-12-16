FROM public.ecr.aws/docker/library/golang:1.19.4-alpine as build

ARG RELEASE
ARG COMMIT
ARG BUILD_TIME
ARG PROJECT=github.com/undeadops/webby/pkg

WORKDIR /app

COPY go.mod .
RUN go mod download

COPY *.go ./
COPY . ./

RUN go build -ldflags "-s -w -X ${PROJECT}/version.Release=${RELEASE} \
			-X ${PROJECT}/version.Commit=${COMMIT} \
			-X ${PROJECT}/version.BuildTime=${BUILD_TIME}" \
			-o webby *.go


FROM public.ecr.aws/docker/library/alpine:3.17

RUN apk --no-cache add ca-certificates
WORKDIR /

COPY --from=build /app/webby /bin/

EXPOSE 5000
CMD ["/bin/webby"]
