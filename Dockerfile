FROM golang:latest

WORKDIR /go/src/github.com/matematik7/camino-go/

COPY go* /
RUN go mod download
RUN go get -u github.com/gobuffalo/packr/packr

COPY . .
RUN GOOS=linux packr build -o /binary

RUN mkdir /deps
# auto figure out cgo dependencies
RUN ldd /binary | tr -s '[:blank:]' '\n' | grep '^/' | xargs -L 1 -I % cp --parents % /deps

FROM scratch

WORKDIR /

COPY --from=0 /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=0 /etc/ssl/certs /etc/ssl/certs
COPY --from=0 /binary /
COPY --from=0 /deps /

EXPOSE 8000

CMD ["/binary"]
