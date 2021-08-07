FROM golang:1.16.7-alpine as build

# Install dependencies
WORKDIR /app
COPY ./go.mod ./go.sum ./
RUN go mod download -x

# Build artifacts
COPY ./cmd ./cmd
COPY ./internal ./internal
COPY ./db ./db

FROM build as install
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go/bin/chat cmd/chat.go

FROM alpine:3.14.0

RUN apk --no-cache add ca-certificates=20191127-r5

# add AWS RDS CA bundle
ADD https://s3.amazonaws.com/rds-downloads/rds-combined-ca-bundle.pem /tmp/rds-ca/aws-rds-ca-bundle.pem
# split the bundle into individual certs (prefixed with xx)
# see http://blog.swwomm.com/2015/02/importing-new-rds-ca-certificate-into.html
SHELL ["/bin/ash", "-eo", "pipefail", "-c"]
RUN cd /tmp/rds-ca && cat aws-rds-ca-bundle.pem|awk 'split_after==1{n++;split_after=0} /-----END CERTIFICATE-----/ {split_after=1} {print > "cert" n ""}' \
    && for CERT in /tmp/rds-ca/cert*; do mv $CERT /usr/local/share/ca-certificates/aws-rds-ca-$(basename $CERT).crt; done \
    && rm -rf /tmp/rds-ca \
    && update-ca-certificates

# Copy our static executable
COPY --from=install /go/bin/chat /go/bin/chat

# Run the binary.
ENTRYPOINT ["/go/bin/chat", "api", "run"]
