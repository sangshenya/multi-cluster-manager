#FROM gcr.io/distroless/static:nonroot
#WORKDIR /
#COPY core-manager /
#USER nonroot:nonroot
#
#ENTRYPOINT ["/core-manager"]

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY agent-manager /
USER nonroot:nonroot

ENTRYPOINT ["/agent-manager"]