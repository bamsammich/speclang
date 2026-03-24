FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /echo_tool ./testdata/self/echo_tool

FROM scratch
COPY --from=build /echo_tool /echo_tool
ENTRYPOINT ["/echo_tool"]
