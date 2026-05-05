FROM golang:1.25-alpine

WORKDIR /app

COPY . .

RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod download
RUN go build -o go-admin main.go

EXPOSE 8080

CMD ["./go-admin"]