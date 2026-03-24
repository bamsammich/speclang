FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./testdata/self/http_server

FROM scratch
COPY --from=build /server /server
EXPOSE 8082
ENTRYPOINT ["/server"]
