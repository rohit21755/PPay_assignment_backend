FROM golang:1.24.3-alpine
# install git and cgo dependencies
RUN apk add --no-cache git 

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o main .
EXPOSE 8000
CMD ["./main"]