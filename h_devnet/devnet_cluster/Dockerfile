FROM rsantev/hydra-client:dev

WORKDIR /app

USER root

RUN apk --no-cache add ca-certificates jq

COPY ./hydra.sh ./

RUN chmod +x /app/hydra.sh

# Expose json-rpc, libp2p, grpc and prometheus ports
EXPOSE 8545 9632 1478 5001

ENTRYPOINT ["./hydra.sh"]
