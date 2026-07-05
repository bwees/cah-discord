FROM alpine:3.20
ARG TARGETPLATFORM
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY $TARGETPLATFORM/cah-discord /app/cah
COPY cards cards/
ENTRYPOINT ["/app/cah"]
