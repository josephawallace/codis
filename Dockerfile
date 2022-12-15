FROM golang:1.19-bullseye

WORKDIR /root

COPY . .

RUN ["go", "install", "github.com/milquellc/codis"]

CMD ["sh", "-c", "codis start peer"]
