FROM rsantev/hydra-client:latest

WORKDIR /app

USER root

RUN apk add --no-cache bash curl

COPY ./scripts/secrets.sh .
COPY ./scripts/genesis.sh .
COPY ./scripts/run.sh .

# Make scripts executable
RUN chmod +x genesis.sh
RUN chmod +x run.sh
RUN chmod +x secrets.sh 

EXPOSE 9632 1478 8545

ENTRYPOINT ["./run.sh"]
