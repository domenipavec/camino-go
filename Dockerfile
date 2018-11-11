FROM golang:latest

WORKDIR /go/src/github.com/matematik7/camino-go/

RUN go get github.com/gobuffalo/packr/...

COPY . .
RUN go get -d -v ./...
RUN go install -v ./...

RUN CGO_ENABLED=0 GOOS=linux packr build

FROM scratch

WORKDIR /

COPY --from=0 /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=0 /etc/ssl/certs /etc/ssl/certs
COPY --from=0 /go/src/github.com/qor/admin/views /app/views/qor
COPY --from=0 /go/src/github.com/matematik7/camino-go/camino-go .

EXPOSE 3000

CMD ["/camino-go"]
