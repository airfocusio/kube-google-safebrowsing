FROM scratch
ENTRYPOINT ["/bin/kube-google-safebrowsing"]
COPY kube-google-safebrowsing /bin/kube-google-safebrowsing
WORKDIR /workdir
