FROM golang:1.19-bullseye

ARG node_role
ENV node_role=${node_role}

WORKDIR /root

COPY . .

RUN ["go", "install", "github.com/milquellc/codis"]

CMD ["sh", "-c", "codis start ${node_role}"]
