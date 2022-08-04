FROM alpine:3.16 AS ca-certificates
RUN apk add ca-certificates

FROM scratch
COPY --from=ca-certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY kube-google-safebrowsing /bin/kube-google-safebrowsing
ENTRYPOINT ["/bin/kube-google-safebrowsing"]
