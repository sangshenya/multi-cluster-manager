FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY core-manager /
USER nonroot:nonroot

ENTRYPOINT ["/core-manager"]

