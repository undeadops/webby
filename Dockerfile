FROM golang:1.9-alpine3.6
WORKDIR /go/src/github.com/undeadops/webby/
COPY . .
RUN GOOS=linux go build -o webby cmd/webby/*.go

FROM alpine:3.6  
RUN apk --no-cache add ca-certificates
WORKDIR /
EXPOSE 5000
COPY --from=0 /go/src/github.com/undeadops/webby/webby .
CMD ["./webby"]  
