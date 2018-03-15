FROM alpine:3.7
COPY alertmanager2hangoutschat /
RUN apk add --no-cache ca-certificates
ENTRYPOINT ["/alertmanager2hangoutschat"]
