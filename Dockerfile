FROM golang:1.13.8 as build-env

RUN mkdir /pinger
WORKDIR /pinger
COPY go.mod . 
COPY go.sum .

RUN go mod download
COPY . .
RUN cd composer && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/composer
RUN cd /pinger && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o /go/bin/pinger 

FROM scratch 
COPY --from=build-env /go/bin/pinger /go/bin/pinger
COPY --from=build-env /go/bin/composer /go/bin/composer
ENTRYPOINT ["/go/bin/pinger"]
