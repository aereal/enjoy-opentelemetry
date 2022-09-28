FROM golang:1.19 as build

WORKDIR /go/src/app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app ./cmd/upstream

FROM gcr.io/distroless/base-debian11
COPY --from=build /app /
CMD ["./app"]
