FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY cah-discord /app/cah
COPY cards cards/
ENTRYPOINT ["/app/cah"]
